package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlipayCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlipayCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlipayCallbackLogic {
	return &AlipayCallbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlipayCallbackLogic) AlipayCallback(in *pb.AlipayCallbackReq) (*pb.AlipayCallbackResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	accepted, err := AlipayCallback(l.ctx, l.svcCtx, tid, in.FormJson)
	if err != nil {
		return nil, err
	}
	return &pb.AlipayCallbackResp{Accepted: accepted}, nil
}
