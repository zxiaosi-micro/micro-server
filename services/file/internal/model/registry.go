// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）。
package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	tenantx.Scoped("file_meta")
}
