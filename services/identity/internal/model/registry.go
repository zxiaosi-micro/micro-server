// tenantx 登记表（tenantaudit CI 静态对账依据，02 §9.4）：
// identity_db 8 表的租户归属声明——scoped 表的 model SQL 必含 tenant_id 条件/列，
// exempt 表必须给理由。登录定位类查询（mobile_hash / provider+openid 全局 UK）在
// 各 custom model 注释中显式说明。

package model

import "github.com/zxiaosi-micro/micro-common/tenantx"

func init() {
	// 租户表：租户根表，自身无 tenant_id 列，管理面天然跨租户。
	tenantx.Exempt("tenant", "租户根表：平台面管理，自身无 tenant_id 列")
	// 其余 7 表全部为租户表。
	tenantx.Scoped("user")
	tenantx.Scoped("org")
	tenantx.Scoped("role")
	tenantx.Scoped("menu")
	tenantx.Scoped("user_role")
	tenantx.Scoped("role_menu")
	tenantx.Scoped("user_sso_binding")
}
