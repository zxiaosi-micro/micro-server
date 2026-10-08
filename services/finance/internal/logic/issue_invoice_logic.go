package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type IssueInvoiceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewIssueInvoiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IssueInvoiceLogic {
	return &IssueInvoiceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *IssueInvoiceLogic) IssueInvoice(in *pb.IssueInvoiceReq) (*pb.IssueInvoiceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	invoiceNo, err := IssueInvoiceInternal(l.ctx, l.svcCtx, tid, in.PaymentNo, in.Title, in.TaxNo, in.Amount, ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	return &pb.IssueInvoiceResp{InvoiceNo: invoiceNo}, nil
}
