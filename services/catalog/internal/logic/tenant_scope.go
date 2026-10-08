// 租户上下文 helper（ADR-08 ③：INSERT/查询显式携带 tenant_id；无上下文 fail-closed）。

package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// mustTenant 业务面租户（BFF authz 注入；后台作业走 tenantx.Skip）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}

// isDupKey MySQL 1062 唯一冲突判定。
func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// dupKeyName 提取 1062 冲突的索引名。
func dupKeyName(err error) string {
	var me *mysql.MySQLError
	if !errors.As(err, &me) {
		return ""
	}
	if i := strings.LastIndex(me.Message, "key '"); i >= 0 {
		return strings.TrimSuffix(me.Message[i+len("key '"):], "'")
	}
	return ""
}
