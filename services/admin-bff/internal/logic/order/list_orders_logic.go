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

type ListOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrdersLogic {
	return &ListOrdersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListOrdersLogic) ListOrders(req *types.OrderListReq) (resp *types.OrderListResp, err error) {
	respOut, err := l.svcCtx.Order.ListOrder(l.ctx, &opb.ListOrderReq{
		Keyword: req.Keyword, Type: req.Type, Status: req.Status,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.OrderListResp{Total: int(respOut.Total)}
	for _, o := range respOut.List {
		out.List = append(out.List, *orderView(o))
	}
	return out, nil
}
