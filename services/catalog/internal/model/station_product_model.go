// station_product 表 custom 覆写（ADR-08）：场站模板头。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StationProductModel = (*customStationProductModel)(nil)

type (
	// StationProductModel 场站模板模型。
	StationProductModel interface {
		// Insert 新建模板（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *StationProduct) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, stationProductId int64) (*StationProduct, error)
		// FindPage 模板列表（keyword 模糊 name）。
		FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*StationProduct, int64, error)
	}

	customStationProductModel struct {
		*defaultStationProductModel
	}
)

// NewStationProductModel returns a model for the database table.
func NewStationProductModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationProductModel {
	return &customStationProductModel{
		defaultStationProductModel: newStationProductModel(conn, c, opts...),
	}
}

func (m *customStationProductModel) FindOne(ctx context.Context, tenantId, stationProductId int64) (*StationProduct, error) {
	var resp StationProduct
	query := fmt.Sprintf("select %s from %s where `station_product_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		stationProductRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, stationProductId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customStationProductModel) FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*StationProduct, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
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

	listQuery := fmt.Sprintf("select %s from %s where %s order by `station_product_id` desc limit ? offset ?",
		stationProductRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*StationProduct
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
