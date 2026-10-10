// tenantx 登记（device_db；tenantaudit CI 对账依据）。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	// 租户表：查询显式带 tenant_id 条件（ADR-08 口径）。
	tenantx.Scoped("device")
	tenantx.Scoped("device_lifecycle_log")
	tenantx.Scoped("device_topology")
	tenantx.Scoped("firmware")
	tenantx.Scoped("ota_task")
	tenantx.Scoped("ota_device")
	tenantx.Scoped("cmd")
	// 平台事件发件箱：消费侧按信封 tenant_id 恢复（eventbus/schema.go 口径）。
	tenantx.Exempt("event_outbox", "平台事件发件箱:消费侧按信封 tenant_id 恢复,自身无业务通用字段(eventbus/schema.go 口径)")
}
