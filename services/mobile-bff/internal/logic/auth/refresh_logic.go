// refresh_logic.go · 轮换式刷新（client 漂移校验在 identity；重用 → 全端注销回 10403）。

package auth

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RefreshLogic) Refresh(req *types.RefreshReq) (resp *types.RefreshResp, err error) {
	out, err := l.svcCtx.Identity.Refresh(l.ctx, &ipb.RefreshReq{
		RefreshToken: req.RefreshToken,
		Client:       req.Client, // OPS_APP|CLIENT_MINI（options 校验后透传）
	})
	if err != nil {
		return nil, err
	}
	return &types.RefreshResp{Tokens: tokenPair(out.Tokens)}, nil
}
