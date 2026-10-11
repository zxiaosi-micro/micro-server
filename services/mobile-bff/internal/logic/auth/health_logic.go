// health_logic.go · 健康检查（identity Ping + TDengine 只读探测；/healthz 深检）。

package auth

import (
	"context"
	"time"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type HealthLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHealthLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HealthLogic {
	return &HealthLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Health 深检：下游认证链路必须通（ok），TDengine 仅影响趋势页（degraded 不阻断存活探针）。
func (l *HealthLogic) Health() (resp *types.HealthResp, err error) {
	checks := map[string]string{}
	if _, err := l.svcCtx.Identity.Ping(l.ctx, &ipb.PingReq{}); err != nil {
		checks["identity"] = "error: " + err.Error()
	} else {
		checks["identity"] = "ok"
	}
	if err := l.svcCtx.Td.Ping(l.ctx); err != nil {
		checks["tdengine"] = "degraded: " + err.Error()
	} else {
		checks["tdengine"] = "ok"
	}
	status := "ok"
	for _, v := range checks {
		if v != "ok" {
			status = "degraded"
		}
	}
	return &types.HealthResp{Status: status, Checks: checks, Time: time.Now().Format(time.RFC3339)}, nil
}
