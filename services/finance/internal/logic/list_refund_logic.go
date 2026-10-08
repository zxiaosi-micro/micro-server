package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRefundLogic {
	return &ListRefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListRefundLogic) ListRefund(in *pb.ListRefundReq) (*pb.ListRefundResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Refund.ListPage(l.ctx, tid, in.Keyword, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListRefundResp{Total: total}
	for _, r := range list {
		resp.List = append(resp.List, buildRefundDetail(r))
	}
	return resp, nil
}
