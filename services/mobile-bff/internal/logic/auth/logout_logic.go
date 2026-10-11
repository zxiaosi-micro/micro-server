// logout_logic.go · 登出（免鉴权组 + BearerProbe 注入 sid）。

package auth

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *LogoutLogic) Logout() (resp *types.SimpleResp, err error) {
	sid := ctxkit.SID(l.ctx)
	if sid == "" {
		return nil, errcode.ErrTokenInvalid
	}
	if _, err := l.svcCtx.Identity.Logout(l.ctx, &ipb.LogoutReq{Sid: sid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
