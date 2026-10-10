// Models model 聚合（station 同款装配根）。
package model

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Models ops_db 全表模型聚合。
type Models struct {
	AlertRule AlertRuleModel
	Alert     AlertModel
	Ticket    TicketModel
	TicketLog TicketLogModel
	SlaTimer  SlaTimerModel
	InsTmpl   InspectionTemplateModel
	InsPlan   InspectionPlanModel
	InsTask   InspectionTaskModel
	InsItem   InspectionItemModel
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
