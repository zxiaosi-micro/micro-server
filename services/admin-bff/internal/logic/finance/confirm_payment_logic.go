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

type ConfirmPaymentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfirmPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmPaymentLogic {
	return &ConfirmPaymentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfirmPaymentLogic) ConfirmPayment(req *types.PaymentConfirmReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.ConfirmPayment(l.ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: req.PaymentNo, ChannelTxnId: req.ChannelTxnId,
		PaidAmount: req.PaidAmount, Source: req.Source,
	})
	return &types.SimpleResp{}, err
}
