package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type ValidateSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateSessionLogic {
	return &ValidateSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ValidateSession 会话状态查询（APISIX/排障用）：存在性 + 生效认证级别。
func (l *ValidateSessionLogic) ValidateSession(in *pb.ValidateSessionReq) (*pb.ValidateSessionResp, error) {
	if in.Sid == "" {
		return nil, errcode.ErrBadRequest.WithMsg("sid 必填")
	}
	sess, err := l.svcCtx.Sessions.Get(l.ctx, in.Sid)
	if err == sessionx.ErrSessionNotFound {
		return &pb.ValidateSessionResp{Valid: false}, nil
	}
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	level, err := l.svcCtx.Sessions.EffectiveLevel(l.ctx, in.Sid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.ValidateSessionResp{
		Valid:  true,
		Uid:    sess.UID,
		Client: sess.Client,
		Level:  int32(level),
	}, nil
}
