// menu 表 custom 覆写（ADR-08）：目录-菜单-按钮-接口四级权限源；
// 查询显式 tenant_id + 软删过滤。菜单集按租户独立（每租户可裁剪）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MenuModel = (*customMenuModel)(nil)

type (
	// MenuModel 菜单模型。
	MenuModel interface {
		// Insert 新建菜单（生成方法全列 INSERT，含 tenant_id）。
		Insert(ctx context.Context, data *Menu) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, menuId int64) (*Menu, error)
		// FindAllByTenant 租户全量菜单（菜单管理页 + 角色授权树；不分页）。
		FindAllByTenant(ctx context.Context, tenantId int64) ([]*Menu, error)
		// FindByPerm 权限码查询（BFF perm 解析器对账用）。
		FindByPerm(ctx context.Context, tenantId int64, permCode string) (*Menu, error)
		// Update 编辑（name/perm_code/path/icon/sort/status）。
		Update(ctx context.Context, tenantId, menuId, parentId int64, name string, typ int64,
			permCode, path, icon string, sort, status, updatedBy int64) error
		// SoftDelete 软删（Logic 层校验无子节点 + 联动清 role_menu）。
		SoftDelete(ctx context.Context, tenantId, menuId, updatedBy int64) error
		// CountChildren 子节点数（删除前校验用）。
		CountChildren(ctx context.Context, tenantId, parentId int64) (int64, error)
	}

	customMenuModel struct {
		*defaultMenuModel
	}
)

// NewMenuModel returns a model for the database table.
func NewMenuModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MenuModel {
	return &customMenuModel{
		defaultMenuModel: newMenuModel(conn, c, opts...),
	}
}

func (m *customMenuModel) FindOne(ctx context.Context, tenantId, menuId int64) (*Menu, error) {
	var resp Menu
	query := fmt.Sprintf("select %s from %s where `menu_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", menuRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, menuId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMenuModel) FindAllByTenant(ctx context.Context, tenantId int64) ([]*Menu, error) {
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `deleted_at` is null order by `sort`, `menu_id`", menuRows, m.table)
	var list []*Menu
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customMenuModel) FindByPerm(ctx context.Context, tenantId int64, permCode string) (*Menu, error) {
	var resp Menu
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `perm_code` = ? and `deleted_at` is null limit 1", menuRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, tenantId, permCode)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMenuModel) Update(ctx context.Context, tenantId, menuId, parentId int64, name string, typ int64,
	permCode, path, icon string, sort, status, updatedBy int64) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "update %s set `parent_id` = ?, `name` = ?, `type` = ?, `sort` = ?, `status` = ?, `updated_by` = ?, `updated_at` = ?", m.table)
	args := []any{parentId, name, typ, sort, status,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now()}
	if permCode != "" {
		sb.WriteString(", `perm_code` = ?")
		args = append(args, permCode)
	}
	if path != "" {
		sb.WriteString(", `path` = ?")
		args = append(args, path)
	}
	if icon != "" {
		sb.WriteString(", `icon` = ?")
		args = append(args, icon)
	}
	sb.WriteString(" where `menu_id` = ? and `tenant_id` = ? and `deleted_at` is null")
	args = append(args, menuId, tenantId)
	_, err := m.ExecNoCacheCtx(ctx, sb.String(), args...)
	return err
}

func (m *customMenuModel) CountChildren(ctx context.Context, tenantId, parentId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `parent_id` = ? and `deleted_at` is null", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId, parentId); err != nil {
		return 0, err
	}
	return n, nil
}

func (m *customMenuModel) SoftDelete(ctx context.Context, tenantId, menuId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `status` = 2 where `menu_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, menuId, tenantId)
	return err
}
