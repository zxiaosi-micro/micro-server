package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"time"
)

type UpdateInspectionPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateInspectionPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionPlanLogic {
	return &UpdateInspectionPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateInspectionPlanLogic) UpdateInspectionPlan(in *pb.UpdateInspectionPlanReq) (*pb.UpdateInspectionPlanResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	plan, err := l.svcCtx.Models.InsPlan.FindOneScoped(l.ctx, tid, in.PlanId)
	if err != nil {
		return nil, errInspectNotFound
	}
	fields := map[string]any{"updated_by": ctxkit.UID(l.ctx)}
	if in.Name != "" {
		fields["name"] = in.Name
	}
	if in.TemplateId > 0 {
		fields["template_id"] = in.TemplateId
	}
	if in.CronExpr != "" && in.CronExpr != plan.CronExpr {
		next, err := nextExecAt(time.Now(), in.CronExpr)
		if err != nil {
			return nil, err
		}
		fields["cron_expr"] = in.CronExpr
		fields["next_exec_at"] = next
	}
	if in.Enabled >= 0 {
		fields["enabled"] = in.Enabled
	}
	if err := l.svcCtx.Models.InsPlan.UpdateFields(l.ctx, tid, in.PlanId, fields); err != nil {
		return nil, err
	}
	return &pb.UpdateInspectionPlanResp{}, nil
}
