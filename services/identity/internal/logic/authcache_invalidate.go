package logic

import (
	"context"
	"fmt"

	"micro-server/services/identity/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// invalidateTenantAuthCache 租户级 auth_cache 失效（菜单/权限变更后调用）。
// 快照按 uid 键控——枚举租户全部用户逐个 DEL；用户量大时由 BFF miss 重建兜底。
func invalidateTenantAuthCache(sc *svc.ServiceContext, tenantID int64) {
	var uids []int64
	query := "select `user_id` from `user` where `tenant_id` = ? and `deleted_at` is null"
	if err := sc.Conn.QueryRowsCtx(context.Background(), &uids, query, tenantID); err != nil {
		return
	}
	for _, uid := range uids {
		_ = sc.Sessions.DeleteAuthCache(context.Background(), uid)
	}
}

var _ = sqlx.ErrNotFound
var _ = fmt.Sprintf
