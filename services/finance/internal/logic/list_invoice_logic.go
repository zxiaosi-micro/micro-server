package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInvoiceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInvoiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInvoiceLogic {
	return &ListInvoiceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInvoiceLogic) ListInvoice(in *pb.ListInvoiceReq) (*pb.ListInvoiceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Invoice.ListPage(l.ctx, tid, in.Keyword, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListInvoiceResp{Total: total}
	for _, inv := range list {
		resp.List = append(resp.List, buildInvoiceDetail(inv))
	}
	return resp, nil
}
