package logic

import (
	"context"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentLogic {
	return &GetPaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPaymentLogic) GetPayment(in *pb.GetPaymentReq) (*pb.GetPaymentResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	p, err := l.svcCtx.Models.Payment.FindOneByNo(l.ctx, tid, in.PaymentNo)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errPaymentNotFound
		}
		return nil, err
	}
	return &pb.GetPaymentResp{Payment: buildPaymentDetail(p)}, nil
}
