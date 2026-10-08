// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：finance_db 业务表全部为租户表。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("payment")
	tenantx.Scoped("refund")
	tenantx.Scoped("invoice")
	tenantx.Scoped("reconcile_task")
	tenantx.Scoped("rebate_settlement")
}
