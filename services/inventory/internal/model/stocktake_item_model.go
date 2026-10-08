// stocktake_item 表 custom 覆写（ADR-08）：盘点明细（账面快照/实盘/差异）。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StocktakeItemModel = (*customStocktakeItemModel)(nil)

type (
	// StocktakeItemModel 盘点明细模型。
	StocktakeItemModel interface {
		// Insert 新建明细（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *StocktakeItem) (sql.Result, error)
		// ListByStocktake 盘点单全部明细。
		ListByStocktake(ctx context.Context, tenantId, stocktakeId int64) ([]*StocktakeItem, error)
		// SubmitCount 事务内提交实盘数（按 sku 定位明细）。
		SubmitCount(ctx context.Context, session sqlx.Session, stocktakeId, skuId, countedQty int64) error
		// MarkDiff 审批时回填差异列（留痕）。
		MarkDiff(ctx context.Context, session sqlx.Session, itemId, diffQty int64) error
	}

	customStocktakeItemModel struct {
		*defaultStocktakeItemModel
	}
)

// NewStocktakeItemModel returns a model for the database table.
func NewStocktakeItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StocktakeItemModel {
	return &customStocktakeItemModel{
		defaultStocktakeItemModel: newStocktakeItemModel(conn, c, opts...),
	}
}

func (m *customStocktakeItemModel) ListByStocktake(ctx context.Context, tenantId, stocktakeId int64) ([]*StocktakeItem, error) {
	var list []*StocktakeItem
	query := fmt.Sprintf("select %s from %s where `stocktake_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id` asc",
		stocktakeItemRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, stocktakeId, tenantId)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	return list, nil
}

func (m *customStocktakeItemModel) SubmitCount(ctx context.Context, session sqlx.Session, stocktakeId, skuId, countedQty int64) error {
	query := fmt.Sprintf("update %s set `counted_qty` = ?, `updated_at` = current_timestamp where `stocktake_id` = ? and `sku_id` = ? and `deleted_at` is null", m.table)
	_, err := session.ExecCtx(ctx, query, countedQty, stocktakeId, skuId)
	return err
}

func (m *customStocktakeItemModel) MarkDiff(ctx context.Context, session sqlx.Session, itemId, diffQty int64) error {
	query := fmt.Sprintf("update %s set `diff_qty` = ?, `updated_at` = current_timestamp where `item_id` = ?", m.table)
	_, err := session.ExecCtx(ctx, query, diffQty, itemId)
	return err
}
