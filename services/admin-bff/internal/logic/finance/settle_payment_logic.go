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

type SettlePaymentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlePaymentLogic {
	return &SettlePaymentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SettlePaymentLogic) SettlePayment(req *types.PaymentSettleReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.SettlePayment(l.ctx, &finpb.SettlePaymentReq{
		PaymentNo: req.PaymentNo, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
