// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// notification_db 5 表全部为租户表。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("message")
	tenantx.Scoped("notify_template")
	tenantx.Scoped("notify_channel")
	tenantx.Scoped("outbound_log")
	tenantx.Scoped("user_notify_setting")
}
