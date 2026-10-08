// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// party_db 6 表全部为租户表——model SQL 必含 tenant_id 条件/列。

package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("party")
	tenantx.Scoped("contact")
	tenantx.Scoped("staff")
	tenantx.Scoped("dealer_ext")
	tenantx.Scoped("crm_record")
	tenantx.Scoped("opportunity")
}
