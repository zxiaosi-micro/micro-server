// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：contract_db 业务表全部为租户表。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("contract_template")
	tenantx.Scoped("contract")
	tenantx.Scoped("contract_target")
	tenantx.Scoped("contract_file")
	tenantx.Scoped("warranty")
	tenantx.Scoped("warranty_extension")
	tenantx.Scoped("sla_strategy")
	tenantx.Scoped("contract_sla")
	tenantx.Scoped("claim")
}
