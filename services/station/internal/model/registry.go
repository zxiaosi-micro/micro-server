// tenantx 登记（station_db；tenantaudit CI 对账依据）。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("station")
	tenantx.Scoped("station_device")
	tenantx.Scoped("station_staff")
	tenantx.Scoped("station_topology")
	tenantx.Exempt("event_outbox", "平台事件发件箱:消费侧按信封 tenant_id 恢复,自身无业务通用字段(eventbus/schema.go 口径)")
}
