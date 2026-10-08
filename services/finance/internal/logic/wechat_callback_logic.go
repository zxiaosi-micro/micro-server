package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type WechatCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWechatCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WechatCallbackLogic {
	return &WechatCallbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// WechatCallback 微信 v3 回调：验签 → 解密 → 落账。
func (l *WechatCallbackLogic) WechatCallback(in *pb.WechatCallbackReq) (*pb.WechatCallbackResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	accepted, err := WechatCallback(l.ctx, l.svcCtx, tid, in.HeadersJson, in.Body)
	if err != nil {
		return nil, err
	}
	return &pb.WechatCallbackResp{Accepted: accepted}, nil
}
