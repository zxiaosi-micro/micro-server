package session

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type KickSessionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewKickSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *KickSessionsLogic {
	return &KickSessionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// KickSessions 踢下线（sid 定点 / uid+client 该端 / uid 全端）。
func (l *KickSessionsLogic) KickSessions(req *types.SessionKickReq) (*types.SessionKickResp, error) {
	var uid int64
	if req.UID != "" {
		v, err := strconv.ParseInt(req.UID, 10, 64)
		if err != nil {
			return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
		}
		uid = v
	}
	resp, err := l.svcCtx.Identity.KickSessions(l.ctx, &pb.KickSessionsReq{
		Uid: uid, Client: req.Client, Sid: req.SID,
	})
	if err != nil {
		return nil, err
	}
	return &types.SessionKickResp{Kicked: resp.Kicked}, nil
}
