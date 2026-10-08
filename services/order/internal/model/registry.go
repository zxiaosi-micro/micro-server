// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// order_db 业务表全部为租户表；event_outbox 由 eventbus 统一管理（SQL 在 micro-common，不在此登记）。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("order")
	tenantx.Scoped("order_item")
	tenantx.Scoped("saga")
	tenantx.Scoped("shipment")
	tenantx.Scoped("shipment_trace")
	tenantx.Scoped("return_order")
	tenantx.Scoped("return_item")
}

func init() {
	tenantx.Scoped("shipment_item")
}
