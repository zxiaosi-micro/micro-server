// stock_record 表 custom 覆写（ADR-08）：流水追加式（只 Insert/List）；
// uk_stock_record_biz 幂等键——事务内 InsertInTx 撞 1062 由 Logic 层转 ErrTxnReplay（02 §9.2）。

package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StockRecordModel = (*customStockRecordModel)(nil)

type (
	// StockRecordModel 库存流水模型（追加式）。
	StockRecordModel interface {
		// InsertInTx 事务内写流水（与库存变更同事务，含幂等键唯一约束）。
		InsertInTx(ctx context.Context, session sqlx.Session, data *StockRecord) error
		// ListPage 流水列表（wid/skuId 0=全部；bizType 空=全部）。
		ListPage(ctx context.Context, tenantId, warehouseId, skuId int64, bizType string, page, size int64) ([]*StockRecord, int64, error)
	}

	customStockRecordModel struct {
		*defaultStockRecordModel
	}
)

// NewStockRecordModel returns a model for the database table.
func NewStockRecordModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StockRecordModel {
	return &customStockRecordModel{
		defaultStockRecordModel: newStockRecordModel(conn, c, opts...),
	}
}

func (m *customStockRecordModel) InsertInTx(ctx context.Context, session sqlx.Session, data *StockRecord) error {
	query := "insert into `stock_record` (`record_id`, `inventory_id`, `warehouse_id`, `sku_id`, `biz_type`, `biz_no`, `qty`, `before_available`, `after_available`, `before_locked`, `after_locked`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.RecordId, data.InventoryId, data.WarehouseId, data.SkuId,
		data.BizType, data.BizNo, data.Qty, data.BeforeAvailable, data.AfterAvailable,
		data.BeforeLocked, data.AfterLocked, data.Remark, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customStockRecordModel) ListPage(ctx context.Context, tenantId, warehouseId, skuId int64, bizType string, page, size int64) ([]*StockRecord, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if warehouseId > 0 {
		where += " and `warehouse_id` = ?"
		args = append(args, warehouseId)
	}
	if skuId > 0 {
		where += " and `sku_id` = ?"
		args = append(args, skuId)
	}
	if bizType != "" {
		where += " and `biz_type` = ?"
		args = append(args, bizType)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `record_id` desc limit ? offset ?",
		stockRecordRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*StockRecord
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
