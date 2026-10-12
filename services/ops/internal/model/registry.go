// tenantx 登记表（ops_db；tenantaudit CI 静态对账依据，02 §9.4）：
// ops_db 9 张业务表全部为租户表（告警/工单/SLA/巡检），SQL 显式携带 tenant_id 条件（ADR-08）。
// cron 扫描类查询（SLA 违约扫描/工单超时扫描/巡检生成）在对应 model 处豁免注释说明。

package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("alert")
	tenantx.Scoped("alert_rule")
	tenantx.Scoped("ticket")
	tenantx.Scoped("ticket_log")
	tenantx.Scoped("sla_timer")
	tenantx.Scoped("inspection_plan")
	tenantx.Scoped("inspection_template")
	tenantx.Scoped("inspection_task")
	tenantx.Scoped("inspection_item")
}
