package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPaymentLogic {
	return &ListPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListPaymentLogic) ListPayment(in *pb.ListPaymentReq) (*pb.ListPaymentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Payment.ListPage(l.ctx, tid, in.Keyword, in.Channel, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListPaymentResp{Total: total}
	for _, p := range list {
		resp.List = append(resp.List, buildPaymentDetail(p))
	}
	return resp, nil
}
