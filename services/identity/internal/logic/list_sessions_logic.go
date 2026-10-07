package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListSessionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSessionsLogic {
	return &ListSessionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListSessions 会话管理页：在线会话列表（uid=0 → 操作人自己；client 可选过滤）。
func (l *ListSessionsLogic) ListSessions(in *pb.ListSessionsReq) (*pb.ListSessionsResp, error) {
	uid := in.Uid
	if uid == 0 {
		uid = opUID(l.ctx)
	}
	if uid == 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	sessions, err := l.svcCtx.Sessions.ListByUID(l.ctx, uid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	out := make([]*pb.SessionItem, 0, len(sessions))
	for _, s := range sessions {
		if in.Client != "" && s.Client != in.Client {
			continue
		}
		out = append(out, &pb.SessionItem{
			Sid:       s.SID,
			Client:    s.Client,
			CreatedAt: s.CreatedAt.UnixMilli(),
		})
	}
	return &pb.ListSessionsResp{Sessions: out}, nil
}
