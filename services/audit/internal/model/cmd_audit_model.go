// cmd_audit 表 custom 覆写（ADR-08）：控制指令独立审计流（append-only，S7 指令链路产生）。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ CmdAuditModel = (*customCmdAuditModel)(nil)

type (
	// CmdAuditModel 指令审计模型（append-only）。
	CmdAuditModel interface {
		// Insert 追加指令审计（生成方法；写入口唯一）。
		Insert(ctx context.Context, data *CmdAudit) (sql.Result, error)
		// FindPage 指令审计查询（sn/cmd_id 过滤；新→旧）。
		FindPage(ctx context.Context, sn, cmdId string, page, size int64) ([]*CmdAudit, int64, error)
	}

	customCmdAuditModel struct {
		*defaultCmdAuditModel
	}
)

// NewCmdAuditModel returns a model for the database table.
func NewCmdAuditModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) CmdAuditModel {
	return &customCmdAuditModel{
		defaultCmdAuditModel: newCmdAuditModel(conn, c, opts...),
	}
}

func (m *customCmdAuditModel) FindPage(ctx context.Context, sn, cmdId string, page, size int64) ([]*CmdAudit, int64, error) {
	where := "1=1"
	args := []any{}
	if sn != "" {
		where += " and `sn` = ?"
		args = append(args, sn)
	}
	if cmdId != "" {
		where += " and `cmd_id` = ?"
		args = append(args, cmdId)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `cmd_audit_id` desc limit ? offset ?",
		cmdAuditRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*CmdAudit
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
