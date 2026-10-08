package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SettlePaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettlePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlePaymentLogic {
	return &SettlePaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SettlePaymentLogic) SettlePayment(in *pb.SettlePaymentReq) (*pb.SettlePaymentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := SettlePayment(l.ctx, l.svcCtx, tid, in.PaymentNo, in.Remark); err != nil {
		return nil, err
	}
	return &pb.SettlePaymentResp{}, nil
}
