// ops 域 BFF logic 实现（S7-03；转发 ops RPC + string↔int64 归一，E8）。

package ops

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ospb "micro-server/services/ops/pb"
)

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func i64s(v int64) string { return strconv.FormatInt(v, 10) }

// ---- 告警 ----

func alertView(a *ospb.AlertItem) types.AlertView {
	return types.AlertView{
		AlertId: i64s(a.AlertId), Level: a.Level, Source: a.Source, Metric: a.Metric,
		LastValue: a.LastValue, Comparator: a.Comparator, Threshold: a.Threshold,
		StationId: i64s(a.StationId), StationName: a.StationName,
		DeviceId: i64s(a.DeviceId), Sn: a.Sn, Title: a.Title, Content: a.Content,
		Status: a.Status, AggregateCount: a.AggregateCount, Devices: a.Devices,
		FirstSeenAt: a.FirstSeenAt, LastSeenAt: a.LastSeenAt,
		AckedBy: i64s(a.AckedBy), ClosedTicketId: i64s(a.ClosedTicketId), ClosedReason: a.ClosedReason,
	}
}

// ListAlerts 告警列表。
func (l *ListAlertsLogic) ListAlerts(req *types.AlertListReq) (*types.AlertListResp, error) {
	out, err := l.svcCtx.Ops.ListAlert(l.ctx, &ospb.ListAlertReq{
		Status: req.Status, Level: req.Level, Source: req.Source,
		StationId: parseI64(req.StationId), Page: req.Page, Size: req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.AlertListResp{Total: out.Total, List: []types.AlertView{}}
	for _, a := range out.List {
		resp.List = append(resp.List, alertView(a))
	}
	return resp, nil
}

// GetAlert 告警详情。
func (l *GetAlertLogic) GetAlert(req *types.AlertNoPath) (*types.AlertGetResp, error) {
	out, err := l.svcCtx.Ops.GetAlert(l.ctx, &ospb.GetAlertReq{AlertId: parseI64(req.AlertId)})
	if err != nil {
		return nil, err
	}
	return &types.AlertGetResp{Alert: alertView(out.Alert)}, nil
}

// AckAlert 告警确认。
func (l *AckAlertLogic) AckAlert(req *types.AlertAckReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.AckAlert(l.ctx, &ospb.AckAlertReq{AlertId: parseI64(req.AlertId), Note: req.Note}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// TransferAlert 告警转工单（隐含确认；level→优先级）。
func (l *TransferAlertLogic) TransferAlert(req *types.AlertTransferReq) (*types.AlertTransferResp, error) {
	out, err := l.svcCtx.Ops.TransferAlertToTicket(l.ctx, &ospb.TransferAlertToTicketReq{
		AlertId: parseI64(req.AlertId), RequiredSkills: req.RequiredSkills,
		AutoDispatch: req.AutoDispatch, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.AlertTransferResp{
		TicketId: i64s(out.TicketId), TicketNo: out.TicketNo,
		AssigneeUid: i64s(out.AssigneeUid), AssigneeName: out.AssigneeName,
	}, nil
}

// CloseAlert 告警关闭（前置：已验收工单或人工原因）。
func (l *CloseAlertLogic) CloseAlert(req *types.AlertCloseReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CloseAlert(l.ctx, &ospb.CloseAlertReq{
		AlertId: parseI64(req.AlertId), TicketId: parseI64(req.TicketId), Reason: req.Reason,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ArchiveAlert 告警归档。
func (l *ArchiveAlertLogic) ArchiveAlert(req *types.AlertNoPath) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.ArchiveAlert(l.ctx, &ospb.ArchiveAlertReq{AlertId: parseI64(req.AlertId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// PendingAlerts 告警轮询增量（未读计数 + since_id，FR-OPS-010）。
func (l *PendingAlertsLogic) PendingAlerts(req *types.AlertPendingReq) (*types.AlertPendingResp, error) {
	out, err := l.svcCtx.Ops.PendingAlerts(l.ctx, &ospb.PendingAlertsReq{
		StationId: parseI64(req.StationId), SinceId: parseI64(req.SinceId), Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.AlertPendingResp{UnreadCount: out.UnreadCount, MaxId: i64s(out.MaxId), Items: []types.AlertBriefView{}}
	for _, a := range out.Items {
		resp.Items = append(resp.Items, types.AlertBriefView{
			AlertId: i64s(a.AlertId), Level: a.Level, Source: a.Source, Metric: a.Metric,
			Title: a.Title, StationId: i64s(a.StationId), StationName: a.StationName,
			AggregateCount: a.AggregateCount, Status: a.Status, FirstSeenAt: a.FirstSeenAt,
		})
	}
	return resp, nil
}

// ---- 告警规则 ----

func ruleView(r *ospb.AlertRuleDetail) types.AlertRuleView {
	return types.AlertRuleView{
		RuleId: i64s(r.RuleId), Code: r.Code, Name: r.Name, Metric: r.Metric,
		Comparator: r.Comparator, Threshold: r.Threshold, DurationSec: r.DurationSec,
		Level: r.Level, SuppressWindowSec: r.SuppressWindowSec, Scope: r.Scope,
		Enabled: r.Enabled, CreatedAt: r.CreatedAt,
	}
}

// ListAlertRules 规则列表。
func (l *ListAlertRulesLogic) ListAlertRules(req *types.AlertRuleListReq) (*types.AlertRuleListResp, error) {
	out, err := l.svcCtx.Ops.ListAlertRule(l.ctx, &ospb.ListAlertRuleReq{
		Metric: req.Metric, Level: req.Level, Enabled: req.Enabled, Page: req.Page, Size: req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.AlertRuleListResp{Total: out.Total, List: []types.AlertRuleView{}}
	for _, r := range out.List {
		resp.List = append(resp.List, ruleView(r))
	}
	return resp, nil
}

// CreateAlertRule 规则创建。
func (l *CreateAlertRuleLogic) CreateAlertRule(req *types.AlertRuleCreateReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CreateAlertRule(l.ctx, &ospb.CreateAlertRuleReq{
		Code: req.Code, Name: req.Name, Metric: req.Metric, Comparator: req.Comparator,
		Threshold: req.Threshold, DurationSec: req.DurationSec, Level: req.Level,
		SuppressWindowSec: req.SuppressWindowSec, Scope: req.Scope,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// UpdateAlertRule 规则更新。
func (l *UpdateAlertRuleLogic) UpdateAlertRule(req *types.AlertRuleUpdateReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.UpdateAlertRule(l.ctx, &ospb.UpdateAlertRuleReq{
		RuleId: parseI64(req.RuleId), Name: req.Name, Comparator: req.Comparator,
		Threshold: req.Threshold, DurationSec: req.DurationSec, Level: req.Level,
		SuppressWindowSec: req.SuppressWindowSec, Scope: req.Scope, Enabled: req.Enabled,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// DeleteAlertRule 规则删除。
func (l *DeleteAlertRuleLogic) DeleteAlertRule(req *types.AlertNoPath) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.DeleteAlertRule(l.ctx, &ospb.DeleteAlertRuleReq{RuleId: parseI64(req.AlertId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ---- 工单 ----

func ticketView(t *ospb.TicketItem) types.TicketView {
	return types.TicketView{
		TicketId: i64s(t.TicketId), TicketNo: t.TicketNo, Type: t.Type,
		Title: t.Title, Content: t.Content, Priority: t.Priority,
		StationId: i64s(t.StationId), StationName: t.StationName,
		DeviceId: i64s(t.DeviceId), Sn: t.Sn, AlertId: i64s(t.AlertId),
		InspectionItemId: i64s(t.InspectionItemId), AssigneeUid: i64s(t.AssigneeUid),
		AssigneeName: t.AssigneeName, Status: t.Status, EscalateLevel: t.EscalateLevel,
		DispatchAt: t.DispatchAt, AcceptDeadline: t.AcceptDeadline,
		AcceptedAt: t.AcceptedAt, SubmittedAt: t.SubmittedAt, VerifiedAt: t.VerifiedAt,
		ClosedAt: t.ClosedAt, CancelReason: t.CancelReason, CreatedBy: i64s(t.CreatedBy),
		CreatedAt: t.CreatedAt,
	}
}

// ListTickets 工单列表。
func (l *ListTicketsLogic) ListTickets(req *types.TicketListReq) (*types.TicketListResp, error) {
	out, err := l.svcCtx.Ops.ListTicket(l.ctx, &ospb.ListTicketReq{
		Status: req.Status, Type: req.Type, StationId: parseI64(req.StationId),
		AssigneeUid: parseI64(req.AssigneeUid), Page: req.Page, Size: req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.TicketListResp{Total: out.Total, List: []types.TicketView{}}
	for _, t := range out.List {
		resp.List = append(resp.List, ticketView(t))
	}
	return resp, nil
}

// GetTicket 工单详情（含日志）。
func (l *GetTicketLogic) GetTicket(req *types.TicketNoPath) (*types.TicketGetResp, error) {
	out, err := l.svcCtx.Ops.GetTicket(l.ctx, &ospb.GetTicketReq{TicketId: parseI64(req.TicketId)})
	if err != nil {
		return nil, err
	}
	resp := &types.TicketGetResp{Ticket: ticketView(out.Ticket), Logs: []types.TicketLogView{}}
	for _, g := range out.Logs {
		resp.Logs = append(resp.Logs, types.TicketLogView{
			LogId: i64s(g.LogId), TicketId: i64s(g.TicketId), Action: g.Action,
			FromStatus: g.FromStatus, ToStatus: g.ToStatus,
			OperatorUid: i64s(g.OperatorUid), Note: g.Note, CreatedAt: g.CreatedAt,
		})
	}
	return resp, nil
}

// CreateTicket 工单创建（client_action_id 离线幂等）。
func (l *CreateTicketLogic) CreateTicket(req *types.TicketCreateReq) (*types.TicketCreateResp, error) {
	out, err := l.svcCtx.Ops.CreateTicket(l.ctx, &ospb.CreateTicketReq{
		Type: req.Type, Title: req.Title, Content: req.Content, Priority: req.Priority,
		StationId: parseI64(req.StationId), DeviceId: parseI64(req.DeviceId),
		Sn: req.Sn, AlertId: parseI64(req.AlertId), ClientActionId: req.ClientActionId,
	})
	if err != nil {
		return nil, err
	}
	return &types.TicketCreateResp{TicketId: i64s(out.TicketId), TicketNo: out.TicketNo}, nil
}

// PendingTickets 工单轮询增量（FR-OPS-010）。
func (l *PendingTicketsLogic) PendingTickets(req *types.TicketPendingReq) (*types.TicketPendingResp, error) {
	out, err := l.svcCtx.Ops.PendingTickets(l.ctx, &ospb.PendingTicketsReq{
		StationId: parseI64(req.StationId), AssigneeUid: parseI64(req.AssigneeUid),
		Status: req.Status, SinceId: parseI64(req.SinceId), Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.TicketPendingResp{UnreadCount: out.UnreadCount, MaxId: i64s(out.MaxId), Items: []types.TicketBriefView{}}
	for _, t := range out.Items {
		resp.Items = append(resp.Items, types.TicketBriefView{
			TicketId: i64s(t.TicketId), TicketNo: t.TicketNo, Type: t.Type, Title: t.Title,
			Priority: t.Priority, StationId: i64s(t.StationId), StationName: t.StationName,
			Status: t.Status, EscalateLevel: t.EscalateLevel, CreatedAt: t.CreatedAt,
		})
	}
	return resp, nil
}

// MatchStaff 派单候选（区域+技能）。
func (l *MatchStaffLogic) MatchStaff(req *types.TicketMatchStaffReq) (*types.TicketMatchStaffResp, error) {
	out, err := l.svcCtx.Ops.MatchStaff(l.ctx, &ospb.MatchStaffReq{
		StationId: parseI64(req.StationId), RequiredSkills: req.RequiredSkills,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.TicketMatchStaffResp{List: []types.StaffCandidateView{}}
	for _, s := range out.List {
		resp.List = append(resp.List, types.StaffCandidateView{
			StaffId: i64s(s.StaffId), UserId: i64s(s.UserId), Name: s.Name,
			StaffType: s.StaffType, SkillTags: s.SkillTags, WorkRegion: s.WorkRegion,
		})
	}
	return resp, nil
}

// DispatchTicket 工单派单。
func (l *DispatchTicketLogic) DispatchTicket(req *types.TicketDispatchReq) (*types.TicketDispatchResp, error) {
	out, err := l.svcCtx.Ops.DispatchTicket(l.ctx, &ospb.DispatchTicketReq{
		TicketId: parseI64(req.TicketId), AssigneeUid: parseI64(req.AssigneeUid),
		AcceptTimeoutMin: req.AcceptTimeoutMin,
	})
	if err != nil {
		return nil, err
	}
	return &types.TicketDispatchResp{
		AssigneeUid: i64s(out.AssigneeUid), AssigneeName: out.AssigneeName,
		AcceptDeadline: out.AcceptDeadline,
	}, nil
}

// AcceptTicket 接单。
func (l *AcceptTicketLogic) AcceptTicket(req *types.TicketActionReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.AcceptTicket(l.ctx, &ospb.AcceptTicketReq{TicketId: parseI64(req.TicketId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// StartTicket 开工。
func (l *StartTicketLogic) StartTicket(req *types.TicketActionReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.StartTicket(l.ctx, &ospb.StartTicketReq{TicketId: parseI64(req.TicketId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// SubmitTicket 提交。
func (l *SubmitTicketLogic) SubmitTicket(req *types.TicketSubmitReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.SubmitTicket(l.ctx, &ospb.SubmitTicketReq{TicketId: parseI64(req.TicketId), Result: req.Result}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// VerifyTicket 验收（pass=false 退回）。
func (l *VerifyTicketLogic) VerifyTicket(req *types.TicketVerifyReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.VerifyTicket(l.ctx, &ospb.VerifyTicketReq{TicketId: parseI64(req.TicketId), Pass: req.Pass, Remark: req.Remark}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// CloseTicket 关单。
func (l *CloseTicketLogic) CloseTicket(req *types.TicketActionReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CloseTicket(l.ctx, &ospb.CloseTicketReq{TicketId: parseI64(req.TicketId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// CancelTicket 取消。
func (l *CancelTicketLogic) CancelTicket(req *types.TicketCancelReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CancelTicket(l.ctx, &ospb.CancelTicketReq{TicketId: parseI64(req.TicketId), Reason: req.Reason}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ---- SLA ----

// ListSlaStrategies 策略列表（contract 透传）。
func (l *ListSlaStrategiesLogic) ListSlaStrategies(req *types.NoArgsReq) (*types.SlaStrategyListResp, error) {
	out, err := l.svcCtx.Ops.ListSlaStrategy(l.ctx, &ospb.ListSlaStrategyReq{})
	if err != nil {
		return nil, err
	}
	resp := &types.SlaStrategyListResp{List: []types.SlaStrategyView{}}
	for _, s := range out.List {
		resp.List = append(resp.List, types.SlaStrategyView{
			StrategyId: i64s(s.StrategyId), Code: s.Code, Name: s.Name, Level: s.Level,
			ResponseMinutes: int(s.ResponseMinutes), ResolveMinutes: int(s.ResolveMinutes),
		})
	}
	return resp, nil
}

// GetSlaTimer 工单计时器。
func (l *GetSlaTimerLogic) GetSlaTimer(req *types.TicketNoPath) (*types.SlaTimerResp, error) {
	out, err := l.svcCtx.Ops.GetSlaTimer(l.ctx, &ospb.GetSlaTimerReq{TicketId: parseI64(req.TicketId)})
	if err != nil {
		return nil, err
	}
	t := out.Timer
	return &types.SlaTimerResp{Timer: types.SlaTimerView{
		TimerId: i64s(t.TimerId), TicketId: i64s(t.TicketId), StrategyId: i64s(t.StrategyId),
		StrategySnapshot: t.StrategySnapshot, ResponseDueAt: t.ResponseDueAt,
		ResolveDueAt: t.ResolveDueAt, PausedAt: t.PausedAt, PauseReason: t.PauseReason,
		PauseTotalMin: t.PauseTotalMin, EscalateLevel: t.EscalateLevel,
		RespondedAt: t.RespondedAt, ResolvedAt: t.ResolvedAt,
		ResponseBreachAt: t.ResponseBreachAt, ResolveBreachAt: t.ResolveBreachAt,
	}}, nil
}

// PauseSlaTimer 暂停（留痕）。
func (l *PauseSlaTimerLogic) PauseSlaTimer(req *types.SlaPauseReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.PauseSlaTimer(l.ctx, &ospb.PauseSlaTimerReq{TicketId: parseI64(req.TicketId), Reason: req.Reason}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ResumeSlaTimer 恢复（due 顺延）。
func (l *ResumeSlaTimerLogic) ResumeSlaTimer(req *types.TicketActionReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.ResumeSlaTimer(l.ctx, &ospb.ResumeSlaTimerReq{TicketId: parseI64(req.TicketId)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ---- 巡检 ----

// ListInspectionTemplates 模板列表。
func (l *ListInspectionTemplatesLogic) ListInspectionTemplates(req *types.InspectionTemplateListReq) (*types.InspectionTemplateListResp, error) {
	out, err := l.svcCtx.Ops.ListInspectionTemplate(l.ctx, &ospb.ListInspectionTemplateReq{Page: req.Page, Size: req.Size})
	if err != nil {
		return nil, err
	}
	resp := &types.InspectionTemplateListResp{Total: out.Total, List: []types.InspectionTemplateView{}}
	for _, t := range out.List {
		resp.List = append(resp.List, types.InspectionTemplateView{
			TemplateId: i64s(t.TemplateId), Name: t.Name, ItemsSchema: t.ItemsSchema, CreatedAt: t.CreatedAt,
		})
	}
	return resp, nil
}

// CreateInspectionTemplate 模板创建。
func (l *CreateInspectionTemplateLogic) CreateInspectionTemplate(req *types.InspectionTemplateCreateReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CreateInspectionTemplate(l.ctx, &ospb.CreateInspectionTemplateReq{Name: req.Name, ItemsSchema: req.ItemsSchema}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ListInspectionPlans 计划列表。
func (l *ListInspectionPlansLogic) ListInspectionPlans(req *types.InspectionPlanListReq) (*types.InspectionPlanListResp, error) {
	out, err := l.svcCtx.Ops.ListInspectionPlan(l.ctx, &ospb.ListInspectionPlanReq{StationId: parseI64(req.StationId), Page: req.Page, Size: req.Size})
	if err != nil {
		return nil, err
	}
	resp := &types.InspectionPlanListResp{Total: out.Total, List: []types.InspectionPlanView{}}
	for _, p := range out.List {
		resp.List = append(resp.List, types.InspectionPlanView{
			PlanId: i64s(p.PlanId), StationId: i64s(p.StationId), StationName: p.StationName,
			Name: p.Name, TemplateId: i64s(p.TemplateId), CronExpr: p.CronExpr,
			Enabled: p.Enabled, NextExecAt: p.NextExecAt, LastExecAt: p.LastExecAt,
			CreatedAt: p.CreatedAt,
		})
	}
	return resp, nil
}

// CreateInspectionPlan 计划创建。
func (l *CreateInspectionPlanLogic) CreateInspectionPlan(req *types.InspectionPlanCreateReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.CreateInspectionPlan(l.ctx, &ospb.CreateInspectionPlanReq{
		StationId: parseI64(req.StationId), Name: req.Name,
		TemplateId: parseI64(req.TemplateId), CronExpr: req.CronExpr,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// UpdateInspectionPlan 计划更新。
func (l *UpdateInspectionPlanLogic) UpdateInspectionPlan(req *types.InspectionPlanUpdateReq) (*types.SimpleResp, error) {
	if _, err := l.svcCtx.Ops.UpdateInspectionPlan(l.ctx, &ospb.UpdateInspectionPlanReq{
		PlanId: parseI64(req.PlanId), Name: req.Name, CronExpr: req.CronExpr,
		TemplateId: parseI64(req.TemplateId), Enabled: req.Enabled,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ListInspectionTasks 任务列表。
func (l *ListInspectionTasksLogic) ListInspectionTasks(req *types.InspectionTaskListReq) (*types.InspectionTaskListResp, error) {
	out, err := l.svcCtx.Ops.ListInspectionTask(l.ctx, &ospb.ListInspectionTaskReq{
		StationId: parseI64(req.StationId), Status: req.Status, Page: req.Page, Size: req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp := &types.InspectionTaskListResp{Total: out.Total, List: []types.InspectionTaskView{}}
	for _, t := range out.List {
		resp.List = append(resp.List, taskView(t))
	}
	return resp, nil
}

func taskView(t *ospb.InspectionTaskView) types.InspectionTaskView {
	return types.InspectionTaskView{
		TaskId: i64s(t.TaskId), PlanId: i64s(t.PlanId), TemplateId: i64s(t.TemplateId),
		StationId: i64s(t.StationId), StationName: t.StationName, Title: t.Title,
		ScheduledFor: t.ScheduledFor, Status: t.Status, SubmittedBy: i64s(t.SubmittedBy),
		SubmittedAt: t.SubmittedAt, AnomalyCount: t.AnomalyCount, TicketIds: t.TicketIds,
		CreatedAt: t.CreatedAt,
	}
}

// GetInspectionTask 任务详情（含明细）。
func (l *GetInspectionTaskLogic) GetInspectionTask(req *types.TicketNoPath) (*types.InspectionTaskGetResp, error) {
	out, err := l.svcCtx.Ops.GetInspectionTask(l.ctx, &ospb.GetInspectionTaskReq{TaskId: parseI64(req.TicketId)})
	if err != nil {
		return nil, err
	}
	resp := &types.InspectionTaskGetResp{Task: taskView(out.Task), Items: []types.InspectionItemView{}}
	for _, i := range out.Items {
		resp.Items = append(resp.Items, types.InspectionItemView{
			ItemId: i64s(i.ItemId), TaskId: i64s(i.TaskId), Name: i.Name,
			Result: i.Result, Anomalous: i.Anomalous, TicketId: i64s(i.TicketId),
		})
	}
	return resp, nil
}

// SubmitInspectionTask 批量提交（异常项自动转工单）。
func (l *SubmitInspectionTaskLogic) SubmitInspectionTask(req *types.InspectionSubmitReq) (*types.InspectionSubmitResp, error) {
	items := make([]*ospb.InspectionItemSubmit, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &ospb.InspectionItemSubmit{Name: it.Name, Result: it.Result, Anomalous: it.Anomalous})
	}
	out, err := l.svcCtx.Ops.SubmitInspectionTask(l.ctx, &ospb.SubmitInspectionTaskReq{
		TaskId: parseI64(req.TaskId), Items: items,
		Location: req.Location, Photos: req.Photos, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.InspectionSubmitResp{AnomalyCount: out.AnomalyCount, TicketIds: out.TicketIds}, nil
}

// ---- goctl 生成等价的 Logic 结构（goctl 骨架集中定义，方法实现见上） ----

type ListAlertsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertsLogic {
	return &ListAlertsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAlertLogic {
	return &GetAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type AckAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAckAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AckAlertLogic {
	return &AckAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type TransferAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferAlertLogic {
	return &TransferAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CloseAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseAlertLogic {
	return &CloseAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ArchiveAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveAlertLogic {
	return &ArchiveAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type PendingAlertsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPendingAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PendingAlertsLogic {
	return &PendingAlertsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListAlertRulesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAlertRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertRulesLogic {
	return &ListAlertRulesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAlertRuleLogic {
	return &CreateAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type UpdateAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAlertRuleLogic {
	return &UpdateAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type DeleteAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertRuleLogic {
	return &DeleteAlertRuleLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListTicketsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketsLogic {
	return &ListTicketsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTicketLogic {
	return &GetTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTicketLogic {
	return &CreateTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type PendingTicketsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPendingTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PendingTicketsLogic {
	return &PendingTicketsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type MatchStaffLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMatchStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MatchStaffLogic {
	return &MatchStaffLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type DispatchTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDispatchTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DispatchTicketLogic {
	return &DispatchTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type AcceptTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcceptTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptTicketLogic {
	return &AcceptTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type StartTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStartTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StartTicketLogic {
	return &StartTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SubmitTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitTicketLogic {
	return &SubmitTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type VerifyTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTicketLogic {
	return &VerifyTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CloseTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseTicketLogic {
	return &CloseTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CancelTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelTicketLogic {
	return &CancelTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListSlaStrategiesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSlaStrategiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSlaStrategiesLogic {
	return &ListSlaStrategiesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSlaTimerLogic {
	return &GetSlaTimerLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type PauseSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPauseSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PauseSlaTimerLogic {
	return &PauseSlaTimerLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ResumeSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResumeSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResumeSlaTimerLogic {
	return &ResumeSlaTimerLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListInspectionTemplatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionTemplatesLogic {
	return &ListInspectionTemplatesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateInspectionTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInspectionTemplateLogic {
	return &CreateInspectionTemplateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListInspectionPlansLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionPlansLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionPlansLogic {
	return &ListInspectionPlansLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateInspectionPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateInspectionPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateInspectionPlanLogic {
	return &CreateInspectionPlanLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type UpdateInspectionPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateInspectionPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateInspectionPlanLogic {
	return &UpdateInspectionPlanLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListInspectionTasksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionTasksLogic {
	return &ListInspectionTasksLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetInspectionTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInspectionTaskLogic {
	return &GetInspectionTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SubmitInspectionTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitInspectionTaskLogic {
	return &SubmitInspectionTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
