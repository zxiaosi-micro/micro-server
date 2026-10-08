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

type CreateOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateOrderLogic) CreateOrder(req *types.OrderCreateReq) (resp *types.OrderCreateResp, err error) {
	items := make([]*opb.OrderItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &opb.OrderItemInput{
			SkuId: parseI64(it.SkuId), WarehouseId: parseI64(it.WarehouseId),
			Qty: int32(it.Qty), Sn: it.Sn,
		})
	}
	respOut, err := l.svcCtx.Order.CreateOrder(l.ctx, &opb.CreateOrderReq{
		Type: req.Type, BuyerPartyId: parseI64(req.BuyerPartyId), Remark: req.Remark,
		Items: items, PayTimeoutMinutes: int32(req.PayTimeoutMinutes),
	})
	if err != nil {
		return nil, err
	}
	return &types.OrderCreateResp{
		OrderId: strconv.FormatInt(respOut.OrderId, 10), OrderNo: respOut.OrderNo,
		TotalAmount: respOut.TotalAmount, PayExpireAt: respOut.PayExpireAt,
		SagaId: strconv.FormatInt(respOut.SagaId, 10),
	}, nil
}
