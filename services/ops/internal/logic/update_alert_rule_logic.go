package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type UpdateAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertRuleLogic {
	return &UpdateAlertRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAlertRuleLogic) UpdateAlertRule(in *pb.UpdateAlertRuleReq) (*pb.UpdateAlertRuleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Models.AlertRule.FindOneScoped(l.ctx, tid, in.RuleId); err != nil {
		return nil, errRuleNotFound
	}
	fields := map[string]any{
		"name": in.Name, "comparator": in.Comparator, "threshold": in.Threshold,
		"duration_sec": int64(in.DurationSec), "level": in.Level,
		"suppress_window_sec": int64(in.SuppressWindowSec), "updated_by": ctxkit.UID(l.ctx),
	}
	if in.Scope != "" {
		fields["scope"] = in.Scope
	}
	fields["enabled"] = boolToInt64(in.Enabled)
	if err := l.svcCtx.Models.AlertRule.UpdateFields(l.ctx, tid, in.RuleId, fields); err != nil {
		return nil, err
	}
	return &pb.UpdateAlertRuleResp{}, nil
}
