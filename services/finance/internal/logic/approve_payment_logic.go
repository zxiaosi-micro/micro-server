package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ApprovePaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApprovePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovePaymentLogic {
	return &ApprovePaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApprovePaymentLogic) ApprovePayment(in *pb.ApprovePaymentReq) (*pb.ApprovePaymentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := ApprovePayment(l.ctx, l.svcCtx, tid, in.PaymentNo, in.Approve, in.Remark, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.ApprovePaymentResp{}, nil
}
