// views.go model 行 → pb 视图构建（ID 铅笔直传，时间 UnixMilli）。
package logic

import (
	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/pb"
)

func pbRule(r *model.AlertRule) *pb.AlertRuleDetail {
	out := &pb.AlertRuleDetail{
		RuleId:            r.RuleId,
		Code:              r.Code,
		Name:              r.Name,
		Metric:            r.Metric,
		Comparator:        r.Comparator,
		Threshold:         r.Threshold,
		DurationSec:       int32(r.DurationSec),
		Level:             r.Level,
		SuppressWindowSec: int32(r.SuppressWindowSec),
		Enabled:           r.Enabled == 1,
		CreatedAt:         r.CreatedAt.UnixMilli(),
		UpdatedAt:         r.UpdatedAt.UnixMilli(),
	}
	if r.Scope.Valid {
		out.Scope = r.Scope.String
	}
	return out
}

func pbAlert(a *model.Alert) *pb.AlertItem {
	out := &pb.AlertItem{
		AlertId:        a.AlertId,
		Level:          a.Level,
		Source:         a.Source,
		Metric:         a.Metric,
		Comparator:     a.Comparator,
		StationId:      a.StationId,
		StationName:    a.StationName,
		DeviceId:       a.DeviceId,
		Sn:             a.Sn,
		Title:          a.Title,
		Content:        a.Content,
		Status:         a.Status,
		AggregateCount: int32(a.AggregateCount),
		Devices:        a.Devices.String,
		FirstSeenAt:    a.FirstSeenAt.UnixMilli(),
		LastSeenAt:     a.LastSeenAt.UnixMilli(),
		AckedBy:        a.AckedBy.Int64,
		ClosedTicketId: a.ClosedTicketId,
		ClosedReason:   a.ClosedReason.String,
	}
	if a.Threshold.Valid {
		out.Threshold = a.Threshold.Float64
	}
	if a.LastValue.Valid {
		out.LastValue = a.LastValue.Float64
	}
	if a.AckedAt.Valid {
		out.AckedAt = a.AckedAt.Time.UnixMilli()
	}
	if a.ClosedAt.Valid {
		out.ClosedAt = a.ClosedAt.Time.UnixMilli()
	}
	if a.ArchivedAt.Valid {
		out.ArchivedAt = a.ArchivedAt.Time.UnixMilli()
	}
	return out
}

func pbTicket(t *model.Ticket) *pb.TicketItem {
	out := &pb.TicketItem{
		TicketId:         t.TicketId,
		TicketNo:         t.TicketNo,
		Type:             t.Type,
		Title:            t.Title,
		Content:          t.Content,
		Priority:         t.Priority,
		StationId:        t.StationId,
		StationName:      t.StationName,
		DeviceId:         t.DeviceId,
		Sn:               t.Sn,
		AlertId:          t.AlertId,
		InspectionItemId: t.InspectionItemId,
		AssigneeName:     t.AssigneeName,
		Status:           t.Status,
		EscalateLevel:    int32(t.EscalateLevel),
		CancelReason:     t.CancelReason.String,
		CreatedBy:        t.CreatedBy.Int64,
		ClientActionId:   t.ClientActionId.String,
		CreatedAt:        t.CreatedAt.UnixMilli(),
	}
	if t.AssigneeUid.Valid {
		out.AssigneeUid = t.AssigneeUid.Int64
	}
	if t.DispatchAt.Valid {
		out.DispatchAt = t.DispatchAt.Time.UnixMilli()
	}
	if t.AcceptDeadline.Valid {
		out.AcceptDeadline = t.AcceptDeadline.Time.UnixMilli()
	}
	if t.AcceptedAt.Valid {
		out.AcceptedAt = t.AcceptedAt.Time.UnixMilli()
	}
	if t.SubmittedAt.Valid {
		out.SubmittedAt = t.SubmittedAt.Time.UnixMilli()
	}
	if t.VerifiedAt.Valid {
		out.VerifiedAt = t.VerifiedAt.Time.UnixMilli()
	}
	if t.ClosedAt.Valid {
		out.ClosedAt = t.ClosedAt.Time.UnixMilli()
	}
	return out
}

func pbTicketLog(g *model.TicketLog) *pb.TicketLogItem {
	return &pb.TicketLogItem{
		LogId:       g.LogId,
		TicketId:    g.TicketId,
		Action:      g.Action,
		FromStatus:  g.FromStatus,
		ToStatus:    g.ToStatus,
		OperatorUid: g.OperatorUid,
		Note:        g.Note,
		CreatedAt:   g.CreatedAt.UnixMilli(),
	}
}

func pbSlaTimer(t *model.SlaTimer) *pb.SlaTimerView {
	out := &pb.SlaTimerView{
		TimerId:          t.TimerId,
		TicketId:         t.TicketId,
		StrategyId:       t.StrategyId,
		StrategySnapshot: t.StrategySnapshot,
		PauseTotalMin:    int32(t.PauseTotalMin),
		EscalateLevel:    int32(t.EscalateLevel),
	}
	if !t.ResponseDueAt.IsZero() {
		out.ResponseDueAt = t.ResponseDueAt.UnixMilli()
	}
	if !t.ResolveDueAt.IsZero() {
		out.ResolveDueAt = t.ResolveDueAt.UnixMilli()
	}
	if t.PausedAt.Valid {
		out.PausedAt = t.PausedAt.Time.UnixMilli()
	}
	out.PauseReason = t.PauseReason.String
	if t.RespondedAt.Valid {
		out.RespondedAt = t.RespondedAt.Time.UnixMilli()
	}
	if t.ResolvedAt.Valid {
		out.ResolvedAt = t.ResolvedAt.Time.UnixMilli()
	}
	if t.ResponseBreachAt.Valid {
		out.ResponseBreachAt = t.ResponseBreachAt.Time.UnixMilli()
	}
	if t.ResolveBreachAt.Valid {
		out.ResolveBreachAt = t.ResolveBreachAt.Time.UnixMilli()
	}
	return out
}

func pbPlan(p *model.InspectionPlan) *pb.InspectionPlanView {
	out := &pb.InspectionPlanView{
		PlanId:      p.PlanId,
		StationId:   p.StationId,
		StationName: p.StationName,
		Name:        p.Name,
		TemplateId:  p.TemplateId,
		CronExpr:    p.CronExpr,
		Enabled:     p.Enabled == 1,
		NextExecAt:  p.NextExecAt.UnixMilli(),
		CreatedAt:   p.CreatedAt.UnixMilli(),
	}
	if p.LastExecAt.Valid {
		out.LastExecAt = p.LastExecAt.Time.UnixMilli()
	}
	return out
}

func pbTask(t *model.InspectionTask) *pb.InspectionTaskView {
	out := &pb.InspectionTaskView{
		TaskId:       t.TaskId,
		PlanId:       t.PlanId,
		TemplateId:   t.TemplateId,
		StationId:    t.StationId,
		StationName:  t.StationName,
		Title:        t.Title,
		Status:       t.Status,
		SubmittedBy:  t.SubmittedBy.Int64,
		AnomalyCount: int32(t.AnomalyCount),
		TicketIds:    t.TicketIds.String,
		CreatedAt:    t.CreatedAt.UnixMilli(),
	}
	if !t.ScheduledFor.IsZero() {
		out.ScheduledFor = t.ScheduledFor.UnixMilli()
	}
	if t.SubmittedAt.Valid {
		out.SubmittedAt = t.SubmittedAt.Time.UnixMilli()
	}
	return out
}

func pbInsItem(i *model.InspectionItem) *pb.InspectionItemView {
	return &pb.InspectionItemView{
		ItemId:    i.ItemId,
		TaskId:    i.TaskId,
		Name:      i.Name,
		Result:    i.Result.String,
		Anomalous: i.Anomalous == 1,
		TicketId:  i.TicketId,
	}
}
