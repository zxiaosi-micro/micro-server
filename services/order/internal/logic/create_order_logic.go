package logic

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	catpb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrder Saga 步骤1：创建订单（order+items+saga 同事务，失败本地回滚）。
// 单价快照：catalog.ListPrice latest（服务端权威，不信任客户端价格）。
func (l *CreateOrderLogic) CreateOrder(in *pb.CreateOrderReq) (*pb.CreateOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Type != "DEVICE" && in.Type != "STATION" && in.Type != "PURCHASE" && in.Type != "RETURN" {
		return nil, errOrderTypeBad
	}
	if len(in.Items) == 0 {
		return nil, errItemsRequired
	}

	// 价格快照（catalog 价目 latest RETAIL；金额走分）
	type snapshot struct {
		item      *pb.OrderItemInput
		unitCents int64
	}
	snaps := make([]snapshot, 0, len(in.Items))
	var totalCents int64
	for _, it := range in.Items {
		if it.SkuId <= 0 || it.WarehouseId <= 0 || it.Qty <= 0 {
			return nil, errQtyBad
		}
		cents, err := l.retailCents(tid, it.SkuId)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, snapshot{item: it, unitCents: cents})
		totalCents += cents * int64(it.Qty)
	}

	sc := l.svcCtx
	orderId := sc.Snowflake.MustNextID()
	orderNo := "ORD" + fmt.Sprint(orderId)
	sagaId := sc.Snowflake.MustNextID()
	expireAt := time.Now().Add(time.Duration(effectivePayTimeoutMin(sc)) * time.Minute)
	uid := ctxkit.UID(l.ctx)

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		order := &model.Order{
			OrderId: orderId, OrderNo: orderNo, Type: in.Type, Status: "CREATED",
			TotalAmount: float64(totalCents) / 100,
			TenantId:    tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
		}
		if in.BuyerPartyId > 0 {
			order.BuyerPartyId = sqlInt64(in.BuyerPartyId)
		}
		order.PayExpireAt = sqlTime(expireAt)
		if in.Remark != "" {
			order.Remark = sqlString(in.Remark)
		}
		if err := sc.Models.Order.InsertTx(ctx, session, order); err != nil {
			return err
		}
		for _, s := range snaps {
			item := &model.OrderItem{
				ItemId: sc.Snowflake.MustNextID(), OrderId: orderId,
				SkuId: s.item.SkuId, WarehouseId: s.item.WarehouseId, Qty: int64(s.item.Qty),
				UnitPrice: float64(s.unitCents) / 100,
				Amount:    float64(s.unitCents*int64(s.item.Qty)) / 100,
				TenantId:  tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
			}
		if s.item.Sn != "" {
			item.Sn = sqlString(s.item.Sn)
		}
			if err := sc.Models.OrderItem.InsertTx(ctx, session, item); err != nil {
				return err
			}
		}
		saga := &model.Saga{
			SagaId: sagaId, OrderId: orderId, OrderNo: orderNo, OrderType: in.Type,
			CurrentStep: stepCreate, Status: "RUNNING",
			TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
		}
		saga.NextRetryAt = sqlTime(time.Now()) // 立即到期：异步推进失败时 cron 兜底（ADR-09）
		if err := sc.Models.Saga.InsertTx(ctx, session, saga); err != nil {
			return err
		}
		// order_created 事件（与业务同事务，02 §9.3）
		order.PayExpireAt = sqlTime(expireAt)
		return eventbus.Emit(ctx, session, orderCreatedEvent(tid, order))
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errOrderNoUsed
		}
		return nil, err
	}

	l.Infof("订单已创建 order_no=%s type=%s total=%s saga=%d", orderNo, in.Type, centsToAmount(totalCents), sagaId)

	// Saga 推进（异步加速器；失败由 cron 重试扫描兜底——next_retry_at 已置 now）
	KickAdvance(sc, tid, orderNo)

	return &pb.CreateOrderResp{
		OrderId: orderId, OrderNo: orderNo,
		TotalAmount:  centsToAmount(totalCents),
		PayExpireAt:  expireAt.UnixMilli(),
		SagaId:       sagaId,
	}, nil
}

// retailCents 取 SKU 最新零售价（分）。
func (l *CreateOrderLogic) retailCents(tid int64, skuId int64) (int64, error) {
	if l.svcCtx.Catalog == nil {
		return 0, errDownMiss()
	}
	var price *catpb.PriceItem
	err := rpcCall(l.ctx, func(ctx context.Context) error {
		resp, e := l.svcCtx.Catalog.ListPrice(ctx, &catpb.ListPriceReq{
			SkuId: skuId, LatestOnly: true, Page: 1, Size: 50,
		})
		if e != nil {
			return e
		}
		for _, p := range resp.List {
			if p.PriceType == "RETAIL" {
				price = p
				break
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if price == nil {
		return 0, errAmountBad.WithMsg(fmt.Sprintf("SKU %d 无可用价目", skuId))
	}
	return parseCents(price.Amount)
}
