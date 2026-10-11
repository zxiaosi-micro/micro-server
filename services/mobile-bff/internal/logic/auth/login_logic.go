// login_logic.go · 密码登录（运维 App；client 固定 OPS_APP，互斥踢旧/锁定/审计在 identity）。

package auth

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// appClient 运维 App 固定登录端（02 §9.5 client 匹配）。
const appClient = "OPS_APP"

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.LoginResp, err error) {
	out, err := l.svcCtx.Identity.LoginByPassword(l.ctx, &ipb.LoginByPasswordReq{
		Mobile:   req.Mobile,
		Password: req.Password,
		Client:   appClient,
	})
	if err != nil {
		return nil, err
	}
	return &types.LoginResp{
		Tokens:   tokenPair(out.Tokens),
		UID:      i64s(out.Uid),
		Nickname: out.Nickname,
	}, nil
}
