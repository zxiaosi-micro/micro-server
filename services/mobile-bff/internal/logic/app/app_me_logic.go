// app_me_logic.go · 运维 App 我的身份（authz 注入 uid + auth_cache 角色）。

package app

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type AppMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppMeLogic {
	return &AppMeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppMeLogic) AppMe(req *types.NoArgsReq) (resp *types.AppMeResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out, err := l.svcCtx.Identity.GetUser(l.ctx, &ipb.GetUserReq{Uid: uid})
	if err != nil {
		return nil, err
	}
	roles := ctxkit.Roles(l.ctx)
	if roles == nil {
		roles = []string{}
	}
	tenant, _ := ctxkit.Tenant(l.ctx)
	return &types.AppMeResp{
		UID:          i64s(uid),
		Nickname:     out.GetUser().Nickname,
		MobileMasked: out.GetUser().MobileMasked,
		Roles:        roles,
		TenantId:     i64s(tenant),
	}, nil
}
