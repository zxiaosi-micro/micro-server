// audit_log 表 custom 覆写（ADR-08）。
//
// ⚠ 追加式禁改删（01 FR-SYS-001）：本接口刻意不提供 Update/Delete/SoftDelete——
// 审计为 append-only；连软删都不允许（保留 ≥3 年）。
// 平台级审计流：查询面不强制租户过滤（管理员跨租户审查），写入按来源携带 tenant_id。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuditLogModel = (*customAuditLogModel)(nil)

type (
	// AuditLogModel 操作审计模型（append-only）。
	AuditLogModel interface {
		// Insert 追加审计（生成方法；写入口唯一）。
		Insert(ctx context.Context, data *AuditLog) (sql.Result, error)
		// FindPage 审计查询（action/uid/target 过滤；新→旧）。
		FindPage(ctx context.Context, action string, uid int64, targetType, targetId string, page, size int64) ([]*AuditLog, int64, error)
	}

	customAuditLogModel struct {
		*defaultAuditLogModel
	}
)

// NewAuditLogModel returns a model for the database table.
func NewAuditLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) AuditLogModel {
	return &customAuditLogModel{
		defaultAuditLogModel: newAuditLogModel(conn, c, opts...),
	}
}

func (m *customAuditLogModel) FindPage(ctx context.Context, action string, uid int64, targetType, targetId string, page, size int64) ([]*AuditLog, int64, error) {
	where := "1=1"
	args := []any{}
	if action != "" {
		where += " and `action` = ?"
		args = append(args, action)
	}
	if uid > 0 {
		where += " and `uid` = ?"
		args = append(args, uid)
	}
	if targetType != "" {
		where += " and `target_type` = ?"
		args = append(args, targetType)
	}
	if targetId != "" {
		where += " and `target_id` = ?"
		args = append(args, targetId)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `log_id` desc limit ? offset ?",
		auditLogRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*AuditLog
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
