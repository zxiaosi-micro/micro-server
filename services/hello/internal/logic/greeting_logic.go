// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package logic

import (
	"context"
	"strconv"

	"micro-server/services/hello/internal/confcenter"
	"micro-server/services/hello/internal/cronx"
	"micro-server/services/hello/internal/model"
	"micro-server/services/hello/internal/svc"
	"micro-server/services/hello/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// seedTenantID default 租户（tools/seed 种入；模板服务演示用）。
const seedTenantID int64 = 9000000000000000001

type GreetingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGreetingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GreetingLogic {
	return &GreetingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Greeting 模板演示链路：custom model 查询 + configcenter 热更新值 + cron 注册表 lastRun。
// 无租户上下文的模板服务走 tenantx.Skip 显式豁免（02 §9.4：豁免必须显式可审计）。
func (l *GreetingLogic) Greeting() (resp *types.GreetingResp, err error) {
	ctx := tenantx.Skip(l.ctx) // 模板无鉴权链——真实服务由 authz 注入租户，禁 Skip
	g, err := l.svcCtx.Model.FindFirstByTenant(ctx, seedTenantID)
	if err != nil {
		if err == model.ErrNotFound {
			g = nil // 空库：返回默认问候（模板服务可裸跑）
		} else {
			return nil, errcode.Internal.WithCause(err)
		}
	}
	message := "hello, micro (seed greeting 未初始化)"
	if g != nil {
		message = g.Message
	}
	lastRun := cronx.LastRun("hello-heartbeat")
	out := &types.GreetingResp{
		Message:      message,
		PollInterval: confcenter.Current().PollIntervalMs,
	}
	if !lastRun.IsZero() {
		out.CronLastRun = strconv.FormatInt(lastRun.UnixMilli(), 10)
	}
	return out, nil
}
