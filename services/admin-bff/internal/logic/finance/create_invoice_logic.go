// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateInvoiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateInvoiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInvoiceLogic {
	return &CreateInvoiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateInvoiceLogic) CreateInvoice(req *types.InvoiceCreateReq) (resp *types.InvoiceCreateResp, err error) {
	respOut, err := l.svcCtx.Finance.IssueInvoice(l.ctx, &finpb.IssueInvoiceReq{
		PaymentNo: req.PaymentNo, Title: req.Title, TaxNo: req.TaxNo, Amount: req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &types.InvoiceCreateResp{InvoiceNo: respOut.InvoiceNo}, nil
}
