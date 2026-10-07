package session

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListSessionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSessionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSessionsLogic {
	return &ListSessionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ListSessions 在线会话（uid 空=操作人自己；会话管理页与"我的设备"共用）。
func (l *ListSessionsLogic) ListSessions(req *types.SessionListReq) (*types.SessionListResp, error) {
	uid := ctxkit.UID(l.ctx)
	if req.UID != "" {
		v, err := strconv.ParseInt(req.UID, 10, 64)
		if err != nil {
			return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
		}
		uid = v
	}
	resp, err := l.svcCtx.Identity.ListSessions(l.ctx, &pb.ListSessionsReq{Uid: uid, Client: req.Client})
	if err != nil {
		return nil, err
	}
	list := make([]types.SessionItem, 0, len(resp.Sessions))
	for _, s := range resp.Sessions {
		list = append(list, types.SessionItem{
			SID:       s.Sid,
			Client:    s.Client,
			CreatedAt: strconv.FormatInt(s.CreatedAt, 10),
		})
	}
	return &types.SessionListResp{List: list}, nil
}
