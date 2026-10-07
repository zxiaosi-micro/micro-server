package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type KickSessionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewKickSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KickSessionsLogic {
	return &KickSessionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// KickSessions 踢下线：sid 定点 → 单会话；uid+client → 该端全部；仅 uid → 全端注销。
func (l *KickSessionsLogic) KickSessions(in *pb.KickSessionsReq) (*pb.KickSessionsResp, error) {
	var kicked int64
	switch {
	case in.Sid != "":
		if err := l.svcCtx.Sessions.Revoke(l.ctx, in.Sid, sessionx.ReasonKicked); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		kicked = 1
	case in.Uid > 0 && in.Client != "":
		n, err := l.svcCtx.Sessions.RevokeClient(l.ctx, in.Uid, in.Client)
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		kicked = int64(n)
	case in.Uid > 0:
		n, err := l.svcCtx.Sessions.RevokeAll(l.ctx, in.Uid)
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		kicked = int64(n)
	default:
		return nil, errcode.ErrBadRequest.WithMsg("sid 或 uid(+client) 必填其一")
	}
	return &pb.KickSessionsResp{Kicked: kicked}, nil
}
