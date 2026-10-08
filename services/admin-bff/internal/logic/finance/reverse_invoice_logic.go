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

type ReverseInvoiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReverseInvoiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReverseInvoiceLogic {
	return &ReverseInvoiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReverseInvoiceLogic) ReverseInvoice(req *types.InvoiceReverseReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.ReverseInvoice(l.ctx, &finpb.ReverseInvoiceReq{
		InvoiceNo: req.InvoiceNo, Reason: req.Reason,
	})
	return &types.SimpleResp{}, err
}
