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

type CreateShipmentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateShipmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateShipmentLogic {
	return &CreateShipmentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateShipment 发货单（FR-ORD-005）：已支付订单；出库扣减由 Saga 步骤4 完成，发货单是履约载体。
func (l *CreateShipmentLogic) CreateShipment(in *pb.CreateShipmentReq) (*pb.CreateShipmentResp, error) {
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
		return nil, errOrderStatus.WithMsg("订单未支付，不可发货: " + order.Status)
	}

	shipmentId := sc.Snowflake.MustNextID()
	shipmentNo := "SHP" + fmt.Sprint(shipmentId)
	uid := ctxkit.UID(l.ctx)

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		sh := &model.Shipment{
			ShipmentId: shipmentId, ShipmentNo: shipmentNo,
			OrderId: order.OrderId, OrderNo: order.OrderNo,
			WarehouseId: in.WarehouseId, Status: "PENDING",
			TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
		}
		if in.Carrier != "" {
			sh.Carrier = sqlString(in.Carrier)
		}
		if in.TrackingNo != "" {
			sh.TrackingNo = sqlString(in.TrackingNo)
		}
		if err := sc.Models.Shipment.InsertTx(ctx, session, sh); err != nil {
			return err
		}
		for _, si := range in.Items {
			if si.SkuId <= 0 || si.Qty <= 0 {
				return errQtyBad
			}
			item := &model.ShipmentItem{
				ItemId: sc.Snowflake.MustNextID(), ShipmentId: shipmentId,
				SkuId: si.SkuId, Qty: int64(si.Qty),
				TenantId: tid, CreatedBy: uidAsNull(uid), UpdatedBy: uidAsNull(uid),
			}
			if si.Sn != "" {
				item.Sn = sqlString(si.Sn)
			}
			if err := sc.Models.ShipmentItem.InsertTx(ctx, session, item); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errOrderNoUsed.WithMsg("发货单号重复")
		}
		return nil, err
	}

	l.Infof("发货单已建 shipment_no=%s order_no=%s", shipmentNo, in.OrderNo)
	return &pb.CreateShipmentResp{ShipmentId: shipmentId, ShipmentNo: shipmentNo}, nil
}
