// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）。
// audit_log/cmd_audit 是平台级审计流（管理员跨租户审查），但携带 tenant_id 列；
// 按 Exempt 登记理由（查询面跨租户），写入侧仍按来源租户落列。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Exempt("audit_log", "平台审计流:管理员跨租户审查,写入按来源租户落列,查询面不强制过滤")
	tenantx.Exempt("cmd_audit", "平台指令审计流:同 audit_log 口径,查询面按设备维度审查")
}
