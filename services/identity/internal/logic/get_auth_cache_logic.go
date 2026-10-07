package logic

import (
	"context"
	"errors"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type GetAuthCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAuthCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAuthCacheLogic {
	return &GetAuthCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetAuthCache 权限快照读取与刷新源（02 §9.5）：
// Redis 命中直接返回；miss（登录前/被失效）→ 从 DB 重建并回写。
// BFF 每请求直接 GET Redis（authz），本方法供 /auth/me 与排障刷新。
func (l *GetAuthCacheLogic) GetAuthCache(in *pb.GetAuthCacheReq) (*pb.GetAuthCacheResp, error) {
	if in.Uid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	snap, err := l.svcCtx.Sessions.GetAuthCache(l.ctx, in.Uid)
	if errors.Is(err, sessionx.ErrAuthCacheMiss) {
		// 重建：uid → tenant → RBAC 全量 → 回写
		tid, terr := l.svcCtx.Models.User.FindTenantByUID(l.ctx, in.Uid)
		if terr != nil {
			return nil, errUserNotFound
		}
		if err := ensureAuthCache(l.ctx, l.svcCtx, tid, in.Uid); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		snap, err = l.svcCtx.Sessions.GetAuthCache(l.ctx, in.Uid)
	}
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.GetAuthCacheResp{
		Roles:     snap.Roles,
		Perms:     snap.Perms,
		DataScope: snap.DataScope,
		TenantId:  snap.TenantID,
	}, nil
}
