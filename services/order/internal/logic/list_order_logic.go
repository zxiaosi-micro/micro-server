package logic

import (
	"context"

	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrderLogic {
	return &ListOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListOrder 订单列表（轻量：不含明细，明细走 GetOrder）。
func (l *ListOrderLogic) ListOrder(in *pb.ListOrderReq) (*pb.ListOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Order.ListPage(l.ctx, tid, in.Keyword, in.Type, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListOrderResp{Total: total}
	for _, o := range list {
		d, err := buildOrderDetail(l.ctx, l.svcCtx, tid, o, false)
		if err != nil {
			return nil, err
		}
		resp.List = append(resp.List, d)
	}
	return resp, nil
}
