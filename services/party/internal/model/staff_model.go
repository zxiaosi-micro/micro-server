// staff 表 custom 覆写（ADR-08）：租户过滤；skill_tags/work_region 为 S7 派单匹配依据。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StaffModel = (*customStaffModel)(nil)

type (
	// StaffModel 服务商员工模型。
	StaffModel interface {
		// Insert 新建员工（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *Staff) (sql.Result, error)
		// FindPage 员工列表（partyId 0=全部；keyword 模糊 name）。
		FindPage(ctx context.Context, tenantId, partyId int64, keyword string, page, size int64) ([]*Staff, int64, error)
	}

	customStaffModel struct {
		*defaultStaffModel
	}
)

// NewStaffModel returns a model for the database table.
func NewStaffModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StaffModel {
	return &customStaffModel{
		defaultStaffModel: newStaffModel(conn, c, opts...),
	}
}

func (m *customStaffModel) FindPage(ctx context.Context, tenantId, partyId int64, keyword string, page, size int64) ([]*Staff, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if partyId > 0 {
		where += " and `party_id` = ?"
		args = append(args, partyId)
	}
	if keyword != "" {
		where += " and `name` like ?"
		args = append(args, "%"+keyword+"%")
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `staff_id` desc limit ? offset ?",
		staffRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Staff
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
