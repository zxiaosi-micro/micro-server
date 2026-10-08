// inventory 表 custom 覆写（ADR-08 + 02 §6.2 事务样例）：
//   - 四态（可用/锁定/在途/残次）的事务内行锁调整方法——防超卖双保险的 DB 兜底层；
//   - 全部带余量守卫（WHERE ... >= ?）与 tenant_id，affected==0 即库存不足/不足锁定；
//   - 行锁方法 LockByWhSku（SELECT ... FOR UPDATE）供 Logic 层事务内取 before 快照。
//
// ⚠ goctl 重生成只写 _gen.go——本文件覆写不会被覆盖。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ InventoryModel = (*customInventoryModel)(nil)

type (
	// InventoryModel 库存模型（四态）。
	InventoryModel interface {
		// FindOne 按 (warehouse, sku) 取（租户过滤）；未建档 ErrNotFound。
		FindOne(ctx context.Context, tenantId, warehouseId, skuId int64) (*Inventory, error)
		// LockByWhSku 事务内行锁取行（SELECT ... FOR UPDATE，before 快照）。
		LockByWhSku(ctx context.Context, session sqlx.Session, tenantId, warehouseId, skuId int64) (*Inventory, error)
		// InitRowOnDupInTx 建档或入库（uk_inventory_wh_sku 冲突时 available 累加）。
		InitRowOnDupInTx(ctx context.Context, session sqlx.Session, data *Inventory) error
		// AddAvailableInTx 可用增加（入库/释放/盘点调增统一入口由各专用方法承担，本方法备用）。
		AddAvailableInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) error
		// ReserveInTx 预留：available→locked，余量守卫 available >= qty（02 §6.2 样例原样，affected==0 即不足）。
		ReserveInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error)
		// ReleaseInTx 释放：locked→available，守卫 locked >= qty。
		ReleaseInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error)
		// DeductLockedInTx 出库：locked 扣减，守卫 locked >= qty。
		DeductLockedInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error)
		// AdjustAvailableInTx 通用可用调整（盘点差异/备件领用退库；qty 可正可负，负向守卫 available+qty>=0）。
		AdjustAvailableInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error)
		// SetThreshold 设置低库存阈值。
		SetThreshold(ctx context.Context, tenantId, inventoryId, threshold int64, updatedBy int64) error
		// ListPage 库存列表（wid 0=全部；skuId 0=全部；lowOnly 仅 available < threshold）。
		ListPage(ctx context.Context, tenantId, warehouseId, skuId int64, lowOnly bool, page, size int64) ([]*Inventory, int64, error)
		// ListLowStock 低于阈值的库存行（stock_low 事件扫描兜底，供 cron）。
		ListLowStock(ctx context.Context, tenantId int64, limit int64) ([]*Inventory, error)
	}

	customInventoryModel struct {
		*defaultInventoryModel
	}
)

// NewInventoryModel returns a model for the database table.
func NewInventoryModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) InventoryModel {
	return &customInventoryModel{
		defaultInventoryModel: newInventoryModel(conn, c, opts...),
	}
}

func (m *customInventoryModel) FindOne(ctx context.Context, tenantId, warehouseId, skuId int64) (*Inventory, error) {
	var resp Inventory
	query := fmt.Sprintf("select %s from %s where `warehouse_id` = ? and `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		inventoryRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, warehouseId, skuId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customInventoryModel) LockByWhSku(ctx context.Context, session sqlx.Session, tenantId, warehouseId, skuId int64) (*Inventory, error) {
	var resp Inventory
	query := fmt.Sprintf("select %s from %s where `warehouse_id` = ? and `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update",
		inventoryRows, m.table)
	err := session.QueryRowCtx(ctx, &resp, query, warehouseId, skuId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customInventoryModel) InitRowOnDupInTx(ctx context.Context, session sqlx.Session, data *Inventory) error {
	query := "insert into `inventory` (`inventory_id`, `warehouse_id`, `sku_id`, `available`, `locked`, `in_transit`, `defective`, `low_stock_threshold`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, 0, 0, 0, ?, ?, ?, ?) on duplicate key update `available` = `available` + values(`available`), `updated_by` = values(`updated_by`), `updated_at` = current_timestamp"
	_, err := session.ExecCtx(ctx, query, data.InventoryId, data.WarehouseId, data.SkuId, data.Available,
		data.LowStockThreshold, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customInventoryModel) AddAvailableInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) error {
	query := fmt.Sprintf("update %s set `available` = `available` + ?, `updated_at` = current_timestamp where `inventory_id` = ?", m.table)
	_, err := session.ExecCtx(ctx, query, qty, inventoryId)
	return err
}

// ReserveInTx 02 §6.2 事务样例：行锁 + 余量守卫，affected==0 即库存不足。
func (m *customInventoryModel) ReserveInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error) {
	query := fmt.Sprintf("update %s set `available` = `available` - ?, `locked` = `locked` + ?, `updated_at` = current_timestamp where `inventory_id` = ? and `available` >= ?", m.table)
	res, err := session.ExecCtx(ctx, query, qty, qty, inventoryId, qty)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (m *customInventoryModel) ReleaseInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error) {
	query := fmt.Sprintf("update %s set `available` = `available` + ?, `locked` = `locked` - ?, `updated_at` = current_timestamp where `inventory_id` = ? and `locked` >= ?", m.table)
	res, err := session.ExecCtx(ctx, query, qty, qty, inventoryId, qty)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (m *customInventoryModel) DeductLockedInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error) {
	query := fmt.Sprintf("update %s set `locked` = `locked` - ?, `updated_at` = current_timestamp where `inventory_id` = ? and `locked` >= ?", m.table)
	res, err := session.ExecCtx(ctx, query, qty, inventoryId, qty)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (m *customInventoryModel) AdjustAvailableInTx(ctx context.Context, session sqlx.Session, inventoryId int64, qty int64) (int64, error) {
	var res sql.Result
	var err error
	if qty >= 0 {
		query := fmt.Sprintf("update %s set `available` = `available` + ?, `updated_at` = current_timestamp where `inventory_id` = ?", m.table)
		res, err = session.ExecCtx(ctx, query, qty, inventoryId)
	} else {
		// 负向调整守卫：调整后不得为负
		query := fmt.Sprintf("update %s set `available` = `available` + ?, `updated_at` = current_timestamp where `inventory_id` = ? and `available` >= ?", m.table)
		res, err = session.ExecCtx(ctx, query, qty, inventoryId, -qty)
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (m *customInventoryModel) SetThreshold(ctx context.Context, tenantId, inventoryId, threshold int64, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `low_stock_threshold` = ?, `updated_by` = ?, `updated_at` = current_timestamp where `inventory_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, threshold, toNullInt64(updatedBy), inventoryId, tenantId)
	return err
}

func (m *customInventoryModel) ListPage(ctx context.Context, tenantId, warehouseId, skuId int64, lowOnly bool, page, size int64) ([]*Inventory, int64, error) {
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
	if lowOnly {
		where += " and `available` < `low_stock_threshold`"
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `inventory_id` desc limit ? offset ?",
		inventoryRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Inventory
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customInventoryModel) ListLowStock(ctx context.Context, tenantId int64, limit int64) ([]*Inventory, error) {
	var list []*Inventory
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `deleted_at` is null and `low_stock_threshold` > 0 and `available` < `low_stock_threshold` order by `inventory_id` limit ?",
		inventoryRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, tenantId, limit)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	return list, nil
}
