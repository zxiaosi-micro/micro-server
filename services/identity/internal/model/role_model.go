// role 表 custom 覆写（ADR-08）：(tenant_id, code) 唯一；查询显式 tenant_id + 软删过滤。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RoleModel = (*customRoleModel)(nil)

type (
	// RoleModel 角色模型。
	RoleModel interface {
		// Insert 新建角色（生成方法全列 INSERT，含 tenant_id）。
		Insert(ctx context.Context, data *Role) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, roleId int64) (*Role, error)
		// FindPage 角色列表（code/name 模糊，分页）。
		FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*Role, int64, error)
		// Update 编辑（name/data_scope/remark）。
		Update(ctx context.Context, tenantId, roleId int64, name, dataScope, remark string, updatedBy int64) error
		// SoftDelete 软删（Logic 层联动清 user_role/role_menu 绑定 + 失效 auth_cache）。
		SoftDelete(ctx context.Context, tenantId, roleId, updatedBy int64) error
	}

	customRoleModel struct {
		*defaultRoleModel
	}
)

// NewRoleModel returns a model for the database table.
func NewRoleModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RoleModel {
	return &customRoleModel{
		defaultRoleModel: newRoleModel(conn, c, opts...),
	}
}

func (m *customRoleModel) FindOne(ctx context.Context, tenantId, roleId int64) (*Role, error) {
	var resp Role
	query := fmt.Sprintf("select %s from %s where `role_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", roleRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, roleId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customRoleModel) FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*Role, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`code` like ? or `name` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total,
		fmt.Sprintf("select count(*) from %s where %s", m.table, where), args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	query := fmt.Sprintf("select %s from %s where %s order by `role_id` limit ? offset ?",
		roleRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Role
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customRoleModel) Update(ctx context.Context, tenantId, roleId int64, name, dataScope, remark string, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `name` = ?, `data_scope` = ?, `remark` = ?, `updated_by` = ?, `updated_at` = ? where `role_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, name, dataScope, remark,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), roleId, tenantId)
	return err
}

func (m *customRoleModel) SoftDelete(ctx context.Context, tenantId, roleId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ? where `role_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, roleId, tenantId)
	return err
}
