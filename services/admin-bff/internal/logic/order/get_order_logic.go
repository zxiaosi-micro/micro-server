// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package order

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderLogic {
	return &GetOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetOrderLogic) GetOrder(req *types.OrderNoPath) (resp *types.OrderView, err error) {
	respOut, err := l.svcCtx.Order.GetOrder(l.ctx, &opb.GetOrderReq{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	return orderView(respOut.Order), nil
}
