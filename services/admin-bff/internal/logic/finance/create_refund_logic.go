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

type CreateRefundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRefundLogic {
	return &CreateRefundLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateRefundLogic) CreateRefund(req *types.RefundCreateReq) (resp *types.RefundCreateResp, err error) {
	respOut, err := l.svcCtx.Finance.CreateRefund(l.ctx, &finpb.CreateRefundReq{
		PaymentNo: req.PaymentNo, Amount: req.Amount, ReturnNo: req.ReturnNo, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &types.RefundCreateResp{RefundNo: respOut.RefundNo, Status: respOut.Status}, nil
}
