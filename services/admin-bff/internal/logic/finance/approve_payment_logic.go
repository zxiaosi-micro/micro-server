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

type ApprovePaymentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApprovePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovePaymentLogic {
	return &ApprovePaymentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovePaymentLogic) ApprovePayment(req *types.PaymentApproveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.ApprovePayment(l.ctx, &finpb.ApprovePaymentReq{
		PaymentNo: req.PaymentNo, Approve: req.Approve, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
