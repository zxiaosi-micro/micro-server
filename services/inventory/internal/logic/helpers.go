// 租户上下文 helper + 公共助手（ADR-08 ③；口径同 party/catalog）。

package logic

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"

	"micro-server/services/inventory/internal/svc"
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

// opUID 操作人 uid（BFF authz 注入；后台作业为 0）。
func opUID(ctx context.Context) int64 {
	return ctxkit.UID(ctx)
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

// warehouseExists 仓库存在性校验（只读 count，不取行锁）。
func warehouseExists(ctx context.Context, sc *svc.ServiceContext, tid, wid int64) error {
	var n int64
	query := "select count(*) from `warehouse` where `warehouse_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := sc.Conn.QueryRowCtx(ctx, &n, query, wid, tid); err != nil {
		return errcode.Internal.WithCause(err)
	}
	if n == 0 {
		return errWarehouseNotFound
	}
	return nil
}

// clampPage 分页上限收敛（size 上限 100，与前端 usePaged 同源约束）。
func clampPage(page, size int64) (int64, int64) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
