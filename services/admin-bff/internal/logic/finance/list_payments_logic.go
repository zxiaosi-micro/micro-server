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

type ListPaymentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListPaymentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPaymentsLogic {
	return &ListPaymentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPaymentsLogic) ListPayments(req *types.PaymentListReq) (resp *types.PaymentListResp, err error) {
	respOut, err := l.svcCtx.Finance.ListPayment(l.ctx, &finpb.ListPaymentReq{
		Keyword: req.Keyword, Channel: req.Channel, Status: req.Status,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.PaymentListResp{Total: int(respOut.Total)}
	for _, p := range respOut.List {
		out.List = append(out.List, *paymentView(p))
	}
	return out, nil
}
