// app_pending_alerts_logic.go · 告警待办增量（10s 轮询，FR-OPS-010）。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppPendingAlertsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppPendingAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppPendingAlertsLogic {
	return &AppPendingAlertsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppPendingAlertsLogic) AppPendingAlerts(req *types.AppPendingAlertsReq) (resp *types.AppPendingAlertsResp, err error) {
	out, err := l.svcCtx.Ops.PendingAlerts(l.ctx, &opsb.PendingAlertsReq{
		SinceId: parseI64(req.SinceId),
		Limit:   20,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AppPendingAlertsResp{UnreadCount: out.UnreadCount, MaxId: i64s(out.MaxId), Items: []types.AppAlertBrief{}}
	for _, a := range out.Items {
		resp.Items = append(resp.Items, pbAlertBrief(a))
	}
	return resp, nil
}
