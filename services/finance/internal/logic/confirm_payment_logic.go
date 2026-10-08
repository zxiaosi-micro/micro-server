package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmPaymentLogic {
	return &ConfirmPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfirmPaymentLogic) ConfirmPayment(in *pb.ConfirmPaymentReq) (*pb.ConfirmPaymentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	status, err := ConfirmPayment(l.ctx, l.svcCtx, tid, in.PaymentNo, in.ChannelTxnId, in.PaidAmount, in.Source)
	if err != nil {
		return nil, err
	}
	return &pb.ConfirmPaymentResp{Status: status}, nil
}
