// station_product_item 表 custom 覆写（ADR-08）：BOM 明细。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StationProductItemModel = (*customStationProductItemModel)(nil)

type (
	// StationProductItemModel 场站模板 BOM 明细模型。
	StationProductItemModel interface {
		// Insert 新建明细（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *StationProductItem) (sql.Result, error)
		// ListByStation 模板全部明细。
		ListByStation(ctx context.Context, tenantId, stationProductId int64) ([]*StationProductItem, error)
	}

	customStationProductItemModel struct {
		*defaultStationProductItemModel
	}
)

// NewStationProductItemModel returns a model for the database table.
func NewStationProductItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationProductItemModel {
	return &customStationProductItemModel{
		defaultStationProductItemModel: newStationProductItemModel(conn, c, opts...),
	}
}

func (m *customStationProductItemModel) ListByStation(ctx context.Context, tenantId, stationProductId int64) ([]*StationProductItem, error) {
	var list []*StationProductItem
	query := fmt.Sprintf("select %s from %s where `station_product_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id` asc",
		stationProductItemRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, stationProductId, tenantId)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	return list, nil
}
