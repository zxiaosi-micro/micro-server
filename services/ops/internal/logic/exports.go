// logic 导出入口（cronx/consumer/logic RPC 面跨包调用；实现见 kernel_*.go）。
package logic

import (
	"context"
	"time"

	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/internal/svc"

	partyclient "micro-server/services/party/party"
)

// AcceptTimeoutMin 接单超时分钟（confcenter 热调 → 配置兜底）。
func AcceptTimeoutMin(sc *svc.ServiceContext) int { return acceptTimeoutMin(sc) }

// EscalateTicket 接单超时升级（cronx 调用）。
func EscalateTicket(ctx context.Context, sc *svc.ServiceContext, tk *model.Ticket, timeoutMin int) error {
	return escalateTicket(ctx, sc, tk, timeoutMin)
}

// ScanSlaBreaches SLA 超时分级升级扫描（cronx 调用）。
func ScanSlaBreaches(ctx context.Context, sc *svc.ServiceContext) error {
	return scanSlaBreaches(ctx, sc)
}

// GenerateTasksForPlan 到期计划生成巡检任务（cronx 调用）。
func GenerateTasksForPlan(sc *svc.ServiceContext, plan *model.InspectionPlan) error {
	return generateTasksForPlan(sc, plan)
}

// CreateTicketCore 工单创建内核（RPC 面共用；client_action_id 幂等）。
func CreateTicketCore(ctx context.Context, sc *svc.ServiceContext, tid int64, in *model.Ticket) (*model.Ticket, error) {
	return createTicketCore(ctx, sc, tid, in)
}

// DispatchTicketCore 派单内核（uid=0 自动匹配；返回 uid/name/deadline）。
func DispatchTicketCore(ctx context.Context, sc *svc.ServiceContext, tid, ticketId, assigneeUid int64, timeoutMin int) (int64, string, time.Time, error) {
	return dispatchTicketCore(ctx, sc, tid, ticketId, assigneeUid, timeoutMin)
}

// MatchStaffCore 派单候选内核（区域+技能匹配）。
func MatchStaffCore(ctx context.Context, sc *svc.ServiceContext, tid, stationId int64, requiredSkills []string) ([]*partyclient.StaffItem, error) {
	return matchStaffCore(ctx, sc, tid, stationId, requiredSkills)
}

// InspectionItemIn 巡检明细提交项（RPC 面 → 内核入参）。
type InspectionItemIn struct {
	Name      string // 巡检项名称
	Result    string // 巡检结果（ok/abnormal/na）
	Anomalous bool   // 是否异常（true 则转 PENDING_DISPATCH 工单）
}

// SubmitInspectionCore 巡检批量提交内核（异常项直生成工单）。
func SubmitInspectionCore(ctx context.Context, sc *svc.ServiceContext, tid, taskId, uid int64, location, photos, remark string, items []InspectionItemIn) (int64, string, error) {
	return submitInspectionCore(ctx, sc, tid, taskId, uid, location, photos, remark, items)
}
