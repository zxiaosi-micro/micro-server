package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type WechatPayURLLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWechatPayURLLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WechatPayURLLogic {
	return &WechatPayURLLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *WechatPayURLLogic) WechatPayURL(in *pb.WechatPayURLReq) (*pb.WechatPayURLResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	url, err := WechatPayURL(l.ctx, l.svcCtx, tid, in.PaymentNo)
	if err != nil {
		return nil, err
	}
	return &pb.WechatPayURLResp{PayUrl: url}, nil
}
