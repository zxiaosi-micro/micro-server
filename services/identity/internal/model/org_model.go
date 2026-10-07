// org 表 custom 覆写（ADR-08）：组织树（parent_id），查询显式 tenant_id + 软删过滤。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrgModel = (*customOrgModel)(nil)

type (
	// OrgModel 组织模型。
	OrgModel interface {
		// Insert 新建组织（生成方法全列 INSERT，含 tenant_id）。
		Insert(ctx context.Context, data *Org) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, orgId int64) (*Org, error)
		// FindAllByTenant 租户全量组织（前端组树；规模有限不分页）。
		FindAllByTenant(ctx context.Context, tenantId int64) ([]*Org, error)
		// Update 编辑（name/parent_id/sort）。
		Update(ctx context.Context, tenantId, orgId, parentId int64, name string, sort, updatedBy int64) error
		// SoftDelete 软删（Logic 层校验无子组织、无挂靠用户）。
		SoftDelete(ctx context.Context, tenantId, orgId, updatedBy int64) error
		// CountChildren 子组织数（删除前校验用）。
		CountChildren(ctx context.Context, tenantId, parentId int64) (int64, error)
	}

	customOrgModel struct {
		*defaultOrgModel
	}
)

// NewOrgModel returns a model for the database table.
func NewOrgModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrgModel {
	return &customOrgModel{
		defaultOrgModel: newOrgModel(conn, c, opts...),
	}
}

func (m *customOrgModel) FindOne(ctx context.Context, tenantId, orgId int64) (*Org, error) {
	var resp Org
	query := fmt.Sprintf("select %s from %s where `org_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", orgRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, orgId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customOrgModel) FindAllByTenant(ctx context.Context, tenantId int64) ([]*Org, error) {
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `deleted_at` is null order by `sort`, `org_id`", orgRows, m.table)
	var list []*Org
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customOrgModel) Update(ctx context.Context, tenantId, orgId, parentId int64, name string, sort, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `parent_id` = ?, `name` = ?, `sort` = ?, `updated_by` = ?, `updated_at` = ? where `org_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, parentId, name, sort,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), orgId, tenantId)
	return err
}

func (m *customOrgModel) CountChildren(ctx context.Context, tenantId, parentId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `parent_id` = ? and `deleted_at` is null", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId, parentId); err != nil {
		return 0, err
	}
	return n, nil
}

func (m *customOrgModel) SoftDelete(ctx context.Context, tenantId, orgId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ? where `org_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, orgId, tenantId)
	return err
}
