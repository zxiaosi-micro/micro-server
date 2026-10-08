package logic

import (
	"context"
	"fmt"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateReturnOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateReturnOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReturnOrderLogic {
	return &CreateReturnOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateReturnOrder 退货申请（FR-ORD-004）：仅已支付订单；明细校验原单；退款金额按单价快照计。
func (l *CreateReturnOrderLogic) CreateReturnOrder(in *pb.CreateReturnOrderReq) (*pb.CreateReturnOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if len(in.Items) == 0 {
		return nil, errItemsRequired
	}
	sc := l.svcCtx

	order, err := sc.Models.Order.FindOneByNo(l.ctx, tid, in.OrderNo)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errOrderNotFound
		}
		return nil, err
	}
	switch order.Status {
	case "PAID", "STOCK_OUT", "CONTRACTED", "DONE":
	default:
		return nil, errOrderStatus.WithMsg("未支付订单不可退货: " + order.Status)
	}

	// 明细校验（SKU 须在原单；数量不超原单）
	items, err := sc.Models.OrderItem.FindByOrder(l.ctx, tid, order.OrderId)
	if err != nil {
		return nil, err
	}
	qtyBySku := map[int64]int64{}
	for _, it := range items {
		qtyBySku[it.SkuId] = it.Qty
	}
	priceBySku := map[int64]int64{}
	for _, it := range items {
		priceBySku[it.SkuId] = floatToCents(it.UnitPrice)
	}
	var refundCents int64
	type retItem struct {
		in *pb.ReturnItemInput
	}
	ris := make([]retItem, 0, len(in.Items))
	for _, ri := range in.Items {
		if ri.SkuId <= 0 || ri.Qty <= 0 {
			return nil, errQtyBad
		}
		orig, ok := qtyBySku[ri.SkuId]
		if !ok || ri.Qty > int32(orig) {
			return nil, errItemNotFound
		}
		refundCents += priceBySku[ri.SkuId] * int64(ri.Qty)
		ris = append(ris, retItem{in: ri})
	}

	returnId := sc.Snowflake.MustNextID()
	returnNo := "RET" + fmt.Sprint(returnId)
	uid := ctxkit.UID(l.ctx)

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		ro := &model.ReturnOrder{
			ReturnId: returnId, ReturnNo: returnNo,
			OrderId: order.OrderId, OrderNo: order.OrderNo,
			Status: "APPLYING", TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
		}
		if in.Reason != "" {
			ro.Reason = sqlString(in.Reason)
		}
		if err := sc.Models.ReturnOrder.InsertTx(ctx, session, ro); err != nil {
			return err
		}
		for _, ri := range ris {
			item := &model.ReturnItem{
				ItemId: sc.Snowflake.MustNextID(), ReturnId: returnId,
				SkuId: ri.in.SkuId, Qty: int64(ri.in.Qty),
				TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
			}
			if ri.in.Sn != "" {
				item.Sn = sqlString(ri.in.Sn)
			}
			if ri.in.Reason != "" {
				item.Reason = sqlString(ri.in.Reason)
			}
			if err := sc.Models.ReturnItem.InsertTx(ctx, session, item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errOrderNoUsed.WithMsg("退货单号重复")
		}
		return nil, err
	}

	l.Infof("退货申请已建 return_no=%s order_no=%s refund=%s", returnNo, in.OrderNo, centsToAmount(refundCents))
	return &pb.CreateReturnOrderResp{ReturnId: returnId, ReturnNo: returnNo}, nil
}
