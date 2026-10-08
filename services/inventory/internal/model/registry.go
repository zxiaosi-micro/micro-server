// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// inventory_db 业务 5 表全部为租户表；event_outbox 为平台基础设施表（无业务通用字段，Exempt）。

package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("warehouse")
	tenantx.Scoped("inventory")
	tenantx.Scoped("stock_record")
	tenantx.Scoped("stocktake")
	tenantx.Scoped("stocktake_item")
	tenantx.Exempt("event_outbox", "平台事件发件箱：消费侧按信封 tenant_id 恢复，自身无业务通用字段（eventbus/schema.go 口径）")
}
