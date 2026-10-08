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

type ListInvoicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListInvoicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInvoicesLogic {
	return &ListInvoicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListInvoicesLogic) ListInvoices(req *types.InvoiceListReq) (resp *types.InvoiceListResp, err error) {
	respOut, err := l.svcCtx.Finance.ListInvoice(l.ctx, &finpb.ListInvoiceReq{
		Keyword: req.Keyword, Status: req.Status, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.InvoiceListResp{Total: int(respOut.Total)}
	for _, v := range respOut.List {
		out.List = append(out.List, types.InvoiceView{
			InvoiceNo: v.InvoiceNo, PaymentNo: v.PaymentNo, OrderNo: v.OrderNo,
			Title: v.Title, TaxNo: v.TaxNo, Amount: v.Amount, Status: v.Status,
			ReverseReason: v.ReverseReason, IssuedAt: v.IssuedAt, CreatedAt: v.CreatedAt,
		})
	}
	return out, nil
}
