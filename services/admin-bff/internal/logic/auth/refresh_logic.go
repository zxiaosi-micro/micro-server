package auth

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Refresh 轮换式刷新（重用 → identity 全端注销并回 10403）。
// 单飞由前端 request.ts 保证；后端 GETDEL 原子消费天然幂等防重放。
func (l *RefreshLogic) Refresh(req *types.RefreshReq) (*types.RefreshResp, error) {
	resp, err := l.svcCtx.Identity.Refresh(l.ctx, &pb.RefreshReq{
		RefreshToken: req.RefreshToken,
		Client:       ctxkitClient,
	})
	if err != nil {
		return nil, err
	}
	return &types.RefreshResp{Tokens: tokenPair(resp.Tokens)}, nil
}
