// Models model 聚合（station 同款装配根）。
package model

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Models ops_db 全表模型聚合。
type Models struct {
	AlertRule AlertRuleModel          // 告警规则（抑制窗口/升级阈值）
	Alert     AlertModel              // 告警（状态机 OPEN→ACK→RESOLVED）
	Ticket    TicketModel             // 工单（PENDING_DISPATCH→…→DONE）
	TicketLog TicketLogModel          // 工单操作留痕
	SlaTimer  SlaTimerModel           // SLA 计时器（超时升级驱动）
	InsTmpl   InspectionTemplateModel // 巡检模板
	InsPlan   InspectionPlanModel     // 巡检计划（cron 生成任务）
	InsTask   InspectionTaskModel     // 巡检任务
	InsItem   InspectionItemModel     // 巡检明细（异常项转工单）
}

// NewModels 装配（conn 共用；ops 表多为扫描/聚合查询，无行缓存）。
func NewModels(conn sqlx.SqlConn) *Models {
	return &Models{
		AlertRule: NewAlertRuleModel(conn),
		Alert:     NewAlertModel(conn),
		Ticket:    NewTicketModel(conn),
		TicketLog: NewTicketLogModel(conn),
		SlaTimer:  NewSlaTimerModel(conn),
		InsTmpl:   NewInspectionTemplateModel(conn),
		InsPlan:   NewInspectionPlanModel(conn),
		InsTask:   NewInspectionTaskModel(conn),
		InsItem:   NewInspectionItemModel(conn),
	}
}
