// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package order

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateShipmentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateShipmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateShipmentLogic {
	return &CreateShipmentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateShipmentLogic) CreateShipment(req *types.ShipmentCreateReq) (resp *types.ShipmentCreateResp, err error) {
	items := make([]*opb.ShipmentItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &opb.ShipmentItemInput{
			SkuId: parseI64(it.SkuId), Qty: int32(it.Qty), Sn: it.Sn,
		})
	}
	respOut, err := l.svcCtx.Order.CreateShipment(l.ctx, &opb.CreateShipmentReq{
		OrderNo: req.OrderNo, WarehouseId: parseI64(req.WarehouseId),
		Items: items, Carrier: req.Carrier, TrackingNo: req.TrackingNo,
	})
	if err != nil {
		return nil, err
	}
	return &types.ShipmentCreateResp{
		ShipmentId: strconv.FormatInt(respOut.ShipmentId, 10), ShipmentNo: respOut.ShipmentNo,
	}, nil
}
