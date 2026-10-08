// product 表 custom 覆写（ADR-08）：租户过滤 + NoCache 查询（口径同 party/party_model.go）。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProductModel = (*customProductModel)(nil)

type (
	// ProductModel 商品模型。
	ProductModel interface {
		// Insert 新建商品（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *Product) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, productId int64) (*Product, error)
		// FindPage 商品列表（keyword 模糊 name；status 0=全部）。
		FindPage(ctx context.Context, tenantId int64, keyword string, status int64, page, size int64) ([]*Product, int64, error)
	}

	customProductModel struct {
		*defaultProductModel
	}
)

// NewProductModel returns a model for the database table.
func NewProductModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ProductModel {
	return &customProductModel{
		defaultProductModel: newProductModel(conn, c, opts...),
	}
}

func (m *customProductModel) FindOne(ctx context.Context, tenantId, productId int64) (*Product, error) {
	var resp Product
	query := fmt.Sprintf("select %s from %s where `product_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		productRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, productId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customProductModel) FindPage(ctx context.Context, tenantId int64, keyword string, status int64, page, size int64) ([]*Product, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and `name` like ?"
		args = append(args, "%"+keyword+"%")
	}
	if status > 0 {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `product_id` desc limit ? offset ?",
		productRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Product
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
