package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderLogic {
	return &GetOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetOrder 订单详情（含明细）。
func (l *GetOrderLogic) GetOrder(in *pb.GetOrderReq) (*pb.GetOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.OrderId == 0 && in.OrderNo == "" {
		return nil, errOrderNotFound
	}
	var o *model.Order
	if in.OrderId > 0 {
		o, err = l.svcCtx.Models.Order.FindOneScoped(l.ctx, tid, in.OrderId)
	} else {
		o, err = l.svcCtx.Models.Order.FindOneByNo(l.ctx, tid, in.OrderNo)
	}
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errOrderNotFound
		}
		return nil, err
	}
	d, err := buildOrderDetail(l.ctx, l.svcCtx, tid, o, true)
	if err != nil {
		return nil, err
	}
	return &pb.GetOrderResp{Order: d}, nil
}
