// saga / order_item 表 custom 覆写（ADR-08）。
// Saga 推进并发纪律：状态转移全部 CAS（affected=0 → 已被并发推进，幂等跳过）；
// 行锁读 FindOneForUpdateTx 供取消/补偿路径串行化；扫描方法供 cron 推进器（E16 索引命中）。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— saga ——

var _ SagaModel = (*customSagaModel)(nil)

type (
	// SagaModel Saga 状态机模型（02 §9.1）。
	SagaModel interface {
		sagaModel
		// InsertTx 事务内建 Saga（与订单同事务）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Saga) error
		// FindOneScoped 租户过滤取 Saga。
		FindOneScoped(ctx context.Context, tenantId, sagaId int64) (*Saga, error)
		// FindOneByOrderNo 按订单号取（租户过滤）。
		FindOneByOrderNo(ctx context.Context, tenantId int64, orderNo string) (*Saga, error)
		// FindOneForUpdateTx 事务内行锁取（取消/补偿路径）。
		FindOneForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, orderNo string) (*Saga, error)
		// UpdateStepTx 步骤推进 CAS（from→to；affected=0 即并发已推进）。
		UpdateStepTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, from, to int64) error
		// UpdateStatusTx 整体状态转移（RUNNING/DONE/CANCELLED/MANUAL/COMPENSATING）。
		UpdateStatusTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, status, lastErr string) error
		// MarkRetryTx 步骤失败重试登记（退避 next_retry_at；超限由 Logic 置 MANUAL）。
		MarkRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId, retryCount int64, nextRetryAt time.Time, lastErr string) error
		// ClearRetryTx 推进成功后清退避。
		ClearRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error
		// UpdateContextTx 补偿上下文更新（库存锁键/支付单号/合同号）。
		UpdateContextTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, contextJson string) error
		// MarkDueTx 置为立即到期（推进触发的持久化兜底：进程死亡时 cron 扫描接续，ADR-09）。
		MarkDueTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error
		// ResetForRetryTx 人工重推：回 RUNNING、清退避计数、立即到期。
		ResetForRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error
		// FindRetryDue 重试扫描（status=RUNNING 且到期；后台作业无租户上下文）。
		FindRetryDue(ctx context.Context, now time.Time, limit int) ([]*Saga, error)
		// CountByStatus 人工队列/卡单监控计数。
		CountByStatus(ctx context.Context, status string) (int64, error)
	}

	customSagaModel struct {
		*defaultSagaModel
		conn sqlx.SqlConn
	}
)

// NewSagaModel returns a model for the database table.
func NewSagaModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SagaModel {
	return &customSagaModel{
		defaultSagaModel: newSagaModel(conn, c, opts...),
		conn:             conn,
	}
}

func (m *customSagaModel) InsertTx(ctx context.Context, session sqlx.Session, data *Saga) error {
	query := "insert into `saga` (`saga_id`, `order_id`, `order_no`, `order_type`, `current_step`, `status`, `context_json`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.SagaId, data.OrderId, data.OrderNo, data.OrderType,
		data.CurrentStep, data.Status, data.ContextJson, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customSagaModel) FindOneScoped(ctx context.Context, tenantId, sagaId int64) (*Saga, error) {
	var res Saga
	query := "select " + sagaRows + " from `saga` where `saga_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, sagaId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customSagaModel) FindOneByOrderNo(ctx context.Context, tenantId int64, orderNo string) (*Saga, error) {
	var res Saga
	query := "select " + sagaRows + " from `saga` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customSagaModel) FindOneForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, orderNo string) (*Saga, error) {
	var res Saga
	query := "select " + sagaRows + " from `saga` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update"
	if err := session.QueryRowCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customSagaModel) UpdateStepTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, from, to int64) error {
	query := "update `saga` set `current_step` = ? where `saga_id` = ? and `tenant_id` = ? and `current_step` = ?"
	res, err := session.ExecCtx(ctx, query, to, sagaId, tenantId, from)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customSagaModel) UpdateStatusTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, status, lastErr string) error {
	query := "update `saga` set `status` = ?, `next_retry_at` = null, `last_error` = ? where `saga_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, status, lastErr, sagaId, tenantId)
	return err
}

func (m *customSagaModel) MarkRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId, retryCount int64, nextRetryAt time.Time, lastErr string) error {
	query := "update `saga` set `retry_count` = ?, `next_retry_at` = ?, `last_error` = ? where `saga_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, retryCount, nextRetryAt, lastErr, sagaId, tenantId)
	return err
}

func (m *customSagaModel) ClearRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error {
	query := "update `saga` set `retry_count` = 0, `next_retry_at` = null, `last_error` = '' where `saga_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, sagaId, tenantId)
	return err
}

func (m *customSagaModel) UpdateContextTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64, contextJson string) error {
	query := "update `saga` set `context_json` = ? where `saga_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, contextJson, sagaId, tenantId)
	return err
}

func (m *customSagaModel) MarkDueTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error {
	query := "update `saga` set `next_retry_at` = now(3) where `saga_id` = ? and `tenant_id` = ? and `status` = 'RUNNING'"
	_, err := session.ExecCtx(ctx, query, sagaId, tenantId)
	return err
}

func (m *customSagaModel) ResetForRetryTx(ctx context.Context, session sqlx.Session, tenantId, sagaId int64) error {
	query := "update `saga` set `status` = 'RUNNING', `retry_count` = 0, `next_retry_at` = now(3), `last_error` = '' where `saga_id` = ? and `tenant_id` = ?"
	res, err := session.ExecCtx(ctx, query, sagaId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customSagaModel) FindRetryDue(ctx context.Context, now time.Time, limit int) ([]*Saga, error) {
	var list []*Saga
	query := "select " + sagaRows + " from `saga` where `status` = 'RUNNING' and `next_retry_at` is not null and `next_retry_at` <= ? and `deleted_at` is null order by `next_retry_at` limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, now, limit); err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return list, nil
}

func (m *customSagaModel) CountByStatus(ctx context.Context, status string) (int64, error) {
	var cnt int64
	query := "select count(*) from `saga` where `status` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &cnt, query, status); err != nil {
		return 0, err
	}
	return cnt, nil
}

// —— order_item ——

var _ OrderItemModel = (*customOrderItemModel)(nil)

type (
	// OrderItemModel 订单项模型（单价快照 + 出库确认回写）。
	OrderItemModel interface {
		orderItemModel
		// InsertTx 事务内写订单项（与订单同事务）。
		InsertTx(ctx context.Context, session sqlx.Session, data *OrderItem) error
		// FindByOrderTx 事务内读订单项（Saga 推进快照）。
		FindByOrderTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64) ([]*OrderItem, error)
		// FindByOrder 订单项列表。
		FindByOrder(ctx context.Context, tenantId, orderId int64) ([]*OrderItem, error)
		// IncOutQtyTx 出库确认回写（per (order,sku) 幂等：out_qty 累计不越过 qty，affected=0 即重复确认）。
		IncOutQtyTx(ctx context.Context, session sqlx.Session, tenantId, orderId, skuId int64, qty int64) (bool, error)
	}

	customOrderItemModel struct {
		*defaultOrderItemModel
		conn sqlx.SqlConn
	}
)

// NewOrderItemModel returns a model for the database table.
func NewOrderItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrderItemModel {
	return &customOrderItemModel{
		defaultOrderItemModel: newOrderItemModel(conn, c, opts...),
		conn:                  conn,
	}
}

func (m *customOrderItemModel) InsertTx(ctx context.Context, session sqlx.Session, data *OrderItem) error {
	query := "insert into `order_item` (`item_id`, `order_id`, `sku_id`, `sku_name`, `sn`, `warehouse_id`, `qty`, `unit_price`, `amount`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ItemId, data.OrderId, data.SkuId, data.SkuName,
		data.Sn, data.WarehouseId, data.Qty, data.UnitPrice, data.Amount, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customOrderItemModel) FindByOrderTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64) ([]*OrderItem, error) {
	var list []*OrderItem
	query := "select " + orderItemRows + " from `order_item` where `order_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id`"
	if err := session.QueryRowCtx(ctx, &list, query, orderId, tenantId); err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return list, nil
}

func (m *customOrderItemModel) FindByOrder(ctx context.Context, tenantId, orderId int64) ([]*OrderItem, error) {
	var list []*OrderItem
	query := "select " + orderItemRows + " from `order_item` where `order_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, orderId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customOrderItemModel) IncOutQtyTx(ctx context.Context, session sqlx.Session, tenantId, orderId, skuId, qty int64) (bool, error) {
	query := "update `order_item` set `out_qty` = `out_qty` + ? where `order_id` = ? and `sku_id` = ? and `tenant_id` = ? and `out_qty` + ? <= `qty`"
	res, err := session.ExecCtx(ctx, query, qty, orderId, skuId, tenantId, qty)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
