// sku 表 custom 覆写（ADR-08）：延保也是 SKU（type=STANDARD/EXT_WARRANTY）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SkuModel = (*customSkuModel)(nil)

type (
	// SkuModel SKU 模型。
	SkuModel interface {
		// Insert 新建 SKU（生成方法；data.TenantId 必填；code 租户内 UK）。
		Insert(ctx context.Context, data *Sku) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, skuId int64) (*Sku, error)
		// FindOneByCode 按编码取（租户过滤；库内存在性校验用）。
		FindOneByCode(ctx context.Context, tenantId int64, code string) (*Sku, error)
		// FindPage SKU 列表（productId 0=全部；keyword 模糊 name/code）。
		FindPage(ctx context.Context, tenantId, productId int64, keyword string, page, size int64) ([]*Sku, int64, error)
		// CountByProduct 商品下 SKU 数（下架前校验预留）。
		CountByProduct(ctx context.Context, tenantId, productId int64) (int64, error)
		// UpdateStatus 上下架。
		UpdateStatus(ctx context.Context, tenantId, skuId, status int64, updatedBy int64) error
		// SoftDelete 软删。
		SoftDelete(ctx context.Context, tenantId, skuId, updatedBy int64) error
	}

	customSkuModel struct {
		*defaultSkuModel
	}
)

// NewSkuModel returns a model for the database table.
func NewSkuModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SkuModel {
	return &customSkuModel{
		defaultSkuModel: newSkuModel(conn, c, opts...),
	}
}

func (m *customSkuModel) FindOne(ctx context.Context, tenantId, skuId int64) (*Sku, error) {
	var resp Sku
	query := fmt.Sprintf("select %s from %s where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		skuRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, skuId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customSkuModel) FindOneByCode(ctx context.Context, tenantId int64, code string) (*Sku, error) {
	var resp Sku
	query := fmt.Sprintf("select %s from %s where `code` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		skuRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, code, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customSkuModel) FindPage(ctx context.Context, tenantId, productId int64, keyword string, page, size int64) ([]*Sku, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if productId > 0 {
		where += " and `product_id` = ?"
		args = append(args, productId)
	}
	if keyword != "" {
		where += " and (`name` like ? or `code` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `sku_id` desc limit ? offset ?",
		skuRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Sku
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customSkuModel) CountByProduct(ctx context.Context, tenantId, productId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `product_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &n, query, productId, tenantId)
	return n, err
}

func (m *customSkuModel) UpdateStatus(ctx context.Context, tenantId, skuId, status int64, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `status` = ?, `updated_by` = ?, `updated_at` = ? where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, status, toNullInt64(updatedBy), time.Now(), skuId, tenantId)
	return err
}

func (m *customSkuModel) SoftDelete(ctx context.Context, tenantId, skuId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `updated_at` = ? where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(), toNullInt64(updatedBy), time.Now(), skuId, tenantId)
	return err
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}
