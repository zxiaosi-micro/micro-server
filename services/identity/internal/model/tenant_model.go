// tenant 表 custom 覆写（ADR-08）。
// tenant 是租户根表（自身无 tenant_id 列）——按平台共享表走 Exempt 登记豁免（见 registry.go）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TenantModel = (*customTenantModel)(nil)

type (
	// TenantModel 租户模型（平台面：查询天然跨租户，Exempt 登记豁免）。
	TenantModel interface {
		// Insert 新建租户（生成方法全列 INSERT）。
		Insert(ctx context.Context, data *Tenant) (sql.Result, error)
		// FindOne 按 ID 取租户（软删过滤）。
		FindOne(ctx context.Context, tenantId int64) (*Tenant, error)
		// FindByCode 按 tenant_code 取租户。
		FindByCode(ctx context.Context, code string) (*Tenant, error)
		// FindPage 租户列表（code/name 模糊，分页）。
		FindPage(ctx context.Context, keyword string, page, size int64) ([]*Tenant, int64, error)
		// Update 编辑（name/plan/status；quota 走 UpdateQuota）。
		Update(ctx context.Context, tenantId int64, name, plan string, status, updatedBy int64) error
		// UpdateQuota 配额变更（JSON 串）。
		UpdateQuota(ctx context.Context, tenantId int64, quota string, updatedBy int64) error
		// SoftDelete 软删（租户删除是高危操作，Logic 层校验无关联用户）。
		SoftDelete(ctx context.Context, tenantId, updatedBy int64) error
	}

	customTenantModel struct {
		*defaultTenantModel
	}
)

// NewTenantModel returns a model for the database table.
func NewTenantModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) TenantModel {
	return &customTenantModel{
		defaultTenantModel: newTenantModel(conn, c, opts...),
	}
}

func (m *customTenantModel) FindOne(ctx context.Context, tenantId int64) (*Tenant, error) {
	var resp Tenant
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `deleted_at` is null limit 1", tenantRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customTenantModel) FindByCode(ctx context.Context, code string) (*Tenant, error) {
	var resp Tenant
	query := fmt.Sprintf("select %s from %s where `tenant_code` = ? and `deleted_at` is null limit 1", tenantRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, code)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customTenantModel) FindPage(ctx context.Context, keyword string, page, size int64) ([]*Tenant, int64, error) {
	where := "`deleted_at` is null"
	args := []any{}
	if keyword != "" {
		where += " and (`tenant_code` like ? or `name` like ?)"
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
	query := fmt.Sprintf("select %s from %s where %s order by `tenant_id` limit ? offset ?",
		tenantRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Tenant
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customTenantModel) Update(ctx context.Context, tenantId int64, name, plan string, status, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `name` = ?, `plan` = ?, `status` = ?, `updated_by` = ?, `updated_at` = ? where `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, name, plan, status,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), tenantId)
	return err
}

func (m *customTenantModel) UpdateQuota(ctx context.Context, tenantId int64, quota string, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `quota` = ?, `updated_by` = ?, `updated_at` = ? where `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, quota,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), tenantId)
	return err
}

func (m *customTenantModel) SoftDelete(ctx context.Context, tenantId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `status` = 2 where `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, tenantId)
	return err
}
