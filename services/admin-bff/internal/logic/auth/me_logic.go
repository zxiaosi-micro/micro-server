package auth

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

type MeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
	return &MeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Me 当前用户：uid/roles/tenant 来自 authz 注入的 ctx（每请求 auth_cache 快照），
// 权限码取 identity.GetAuthCache（auth_cache 刷新源），详情走 identity.GetUser。
func (l *MeLogic) Me() (*types.MeResp, error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out := &types.MeResp{
		UID:       strconv.FormatInt(uid, 10),
		Roles:     []string{},
		Perms:     []string{},
		DataScope: ctxkit.DataScope(l.ctx),
	}
	if roles := ctxkit.Roles(l.ctx); len(roles) > 0 {
		out.Roles = roles
	}
	ac, err := l.svcCtx.Identity.GetAuthCache(l.ctx, &pb.GetAuthCacheReq{Uid: uid})
	if err == nil {
		out.Perms = ac.Perms
		if ac.TenantId > 0 {
			out.TenantID = strconv.FormatInt(ac.TenantId, 10)
		}
		if out.DataScope == "" {
			out.DataScope = ac.DataScope
		}
	}
	ur, err := l.svcCtx.Identity.GetUser(l.ctx, &pb.GetUserReq{Uid: uid})
	if err == nil && ur.User != nil {
		out.Nickname = ur.User.Nickname
		out.Mobile = ur.User.MobileMasked
	}
	return out, nil
}
