package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateRefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRefundLogic {
	return &CreateRefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateRefundLogic) CreateRefund(in *pb.CreateRefundReq) (*pb.CreateRefundResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	refundNo, err := CreateRefundInternal(l.ctx, l.svcCtx, tid, in.PaymentNo, in.Amount, in.ReturnNo, in.Reason, "MANUAL", ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	r, err := l.svcCtx.Models.Refund.FindOneByNo(l.ctx, tid, refundNo)
	if err != nil {
		return nil, err
	}
	return &pb.CreateRefundResp{RefundNo: refundNo, Status: r.Status}, nil
}
