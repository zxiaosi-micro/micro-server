// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// catalog_db 6 表全部为租户表——model SQL 必含 tenant_id 条件/列。

package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("product")
	tenantx.Scoped("sku")
	tenantx.Scoped("price")
	tenantx.Scoped("warranty_policy")
	tenantx.Scoped("station_product")
	tenantx.Scoped("station_product_item")
}
