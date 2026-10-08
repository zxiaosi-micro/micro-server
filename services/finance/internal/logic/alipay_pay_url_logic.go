package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlipayPayURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlipayPayURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlipayPayURLLogic {
	return &AlipayPayURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlipayPayURLLogic) AlipayPayURL(in *pb.AlipayPayURLReq) (*pb.AlipayPayURLResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	url, err := AlipayPayURL(l.ctx, l.svcCtx, tid, in.PaymentNo)
	if err != nil {
		return nil, err
	}
	return &pb.AlipayPayURLResp{PayUrl: url}, nil
}
