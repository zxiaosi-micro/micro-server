// shipment / shipment_trace / return_order / return_item 表 custom 覆写（ADR-08）。
// 发货单与轨迹（FR-ORD-005 手工录入基线）；退货单状态机（申请→审批→退款回写，FR-ORD-004）。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— shipment ——

var _ ShipmentModel = (*customShipmentModel)(nil)

type (
	// ShipmentModel 发货单模型。
	ShipmentModel interface {
		shipmentModel
		// InsertTx 事务内建发货单（与出库确认回写可同事务扩展）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Shipment) error
		// FindOneByNo 租户过滤按单号取。
		FindOneByNo(ctx context.Context, tenantId int64, shipmentNo string) (*Shipment, error)
		// CASStatusTx 发货单状态机（PENDING/IN_TRANSIT/SIGNED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, shipmentId int64, from []string, to string) error
		// MarkSignedTx 签收回写。
		MarkSignedTx(ctx context.Context, session sqlx.Session, tenantId, shipmentId int64, signedBy string, signedAt time.Time) error
		// ListByOrder 按订单查发货单。
		ListByOrder(ctx context.Context, tenantId int64, orderNo string) ([]*Shipment, error)
	}

	customShipmentModel struct {
		*defaultShipmentModel
		conn sqlx.SqlConn
	}
)

// NewShipmentModel returns a model for the database table.
func NewShipmentModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ShipmentModel {
	return &customShipmentModel{
		defaultShipmentModel: newShipmentModel(conn, c, opts...),
		conn:                 conn,
	}
}

func (m *customShipmentModel) InsertTx(ctx context.Context, session sqlx.Session, data *Shipment) error {
	query := "insert into `shipment` (`shipment_id`, `shipment_no`, `order_id`, `order_no`, `warehouse_id`, `carrier`, `tracking_no`, `status`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ShipmentId, data.ShipmentNo, data.OrderId, data.OrderNo,
		data.WarehouseId, data.Carrier, data.TrackingNo, data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customShipmentModel) FindOneByNo(ctx context.Context, tenantId int64, shipmentNo string) (*Shipment, error) {
	var res Shipment
	query := "select " + shipmentRows + " from `shipment` where `shipment_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, shipmentNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customShipmentModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, shipmentId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, shipmentId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `shipment` set `status` = ? where `shipment_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customShipmentModel) MarkSignedTx(ctx context.Context, session sqlx.Session, tenantId, shipmentId int64, signedBy string, signedAt time.Time) error {
	query := "update `shipment` set `signed_by` = ?, `signed_at` = ? where `shipment_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, signedBy, signedAt, shipmentId, tenantId)
	return err
}

func (m *customShipmentModel) ListByOrder(ctx context.Context, tenantId int64, orderNo string) ([]*Shipment, error) {
	var list []*Shipment
	query := "select " + shipmentRows + " from `shipment` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null order by `shipment_id` desc"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

// —— shipment_trace ——

var _ ShipmentTraceModel = (*customShipmentTraceModel)(nil)

type (
	// ShipmentTraceModel 物流轨迹模型（手工录入基线，追加式）。
	ShipmentTraceModel interface {
		shipmentTraceModel
		// InsertTx 事务内追加轨迹。
		InsertTx(ctx context.Context, session sqlx.Session, data *ShipmentTrace) error
		// ListByShipment 轨迹时间线（旧→新）。
		ListByShipment(ctx context.Context, tenantId, shipmentId int64) ([]*ShipmentTrace, error)
	}

	customShipmentTraceModel struct {
		*defaultShipmentTraceModel
		conn sqlx.SqlConn
	}
)

// NewShipmentTraceModel returns a model for the database table.
func NewShipmentTraceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ShipmentTraceModel {
	return &customShipmentTraceModel{
		defaultShipmentTraceModel: newShipmentTraceModel(conn, c, opts...),
		conn:                      conn,
	}
}

func (m *customShipmentTraceModel) InsertTx(ctx context.Context, session sqlx.Session, data *ShipmentTrace) error {
	query := "insert into `shipment_trace` (`trace_id`, `shipment_id`, `node`, `description`, `trace_time`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.TraceId, data.ShipmentId, data.Node, data.Description,
		data.TraceTime, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customShipmentTraceModel) ListByShipment(ctx context.Context, tenantId, shipmentId int64) ([]*ShipmentTrace, error) {
	var list []*ShipmentTrace
	query := "select " + shipmentTraceRows + " from `shipment_trace` where `shipment_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `trace_time`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, shipmentId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

// —— return_order ——

var _ ReturnOrderModel = (*customReturnOrderModel)(nil)

type (
	// ReturnOrderModel 退货单模型。
	ReturnOrderModel interface {
		returnOrderModel
		// InsertTx 事务内建退货单。
		InsertTx(ctx context.Context, session sqlx.Session, data *ReturnOrder) error
		// FindOneByNo 租户过滤按单号取。
		FindOneByNo(ctx context.Context, tenantId int64, returnNo string) (*ReturnOrder, error)
		// FindOneByNoForUpdateTx 事务内行锁取（审批并发入口）。
		FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, returnNo string) (*ReturnOrder, error)
		// CASStatusTx 退货状态机（APPLYING/APPROVED/REJECTED/REFUNDED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, from []string, to string) error
		// MarkApprovedTx 审批通过（APPLYING→APPROVED，退款金额落列，事件同事务发）。
		MarkApprovedTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, refundAmount float64) error
		// MarkRejectedTx 审批驳回。
		MarkRejectedTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, reason string) error
		// MarkRefundedTx 退款回写（payment_refunded 事件驱动，FR-ORD-004）。
		MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId int64, returnNo, refundNo string) error
		// ListPage 退货列表。
		ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*ReturnOrder, int64, error)
	}

	customReturnOrderModel struct {
		*defaultReturnOrderModel
		conn sqlx.SqlConn
	}
)

// NewReturnOrderModel returns a model for the database table.
func NewReturnOrderModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ReturnOrderModel {
	return &customReturnOrderModel{
		defaultReturnOrderModel: newReturnOrderModel(conn, c, opts...),
		conn:                    conn,
	}
}

func (m *customReturnOrderModel) InsertTx(ctx context.Context, session sqlx.Session, data *ReturnOrder) error {
	query := "insert into `return_order` (`return_id`, `return_no`, `order_id`, `order_no`, `reason`, `status`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ReturnId, data.ReturnNo, data.OrderId, data.OrderNo,
		data.Reason, data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customReturnOrderModel) FindOneByNo(ctx context.Context, tenantId int64, returnNo string) (*ReturnOrder, error) {
	var res ReturnOrder
	query := "select " + returnOrderRows + " from `return_order` where `return_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, returnNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customReturnOrderModel) FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, returnNo string) (*ReturnOrder, error) {
	var res ReturnOrder
	query := "select " + returnOrderRows + " from `return_order` where `return_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update"
	if err := session.QueryRowCtx(ctx, &res, query, returnNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customReturnOrderModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, returnId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `return_order` set `status` = ? where `return_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customReturnOrderModel) MarkApprovedTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, refundAmount float64) error {
	query := "update `return_order` set `status` = 'APPROVED', `refund_amount` = ? where `return_id` = ? and `tenant_id` = ? and `status` = 'APPLYING'"
	res, err := session.ExecCtx(ctx, query, refundAmount, returnId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customReturnOrderModel) MarkRejectedTx(ctx context.Context, session sqlx.Session, tenantId, returnId int64, reason string) error {
	query := "update `return_order` set `status` = 'REJECTED', `reject_reason` = ? where `return_id` = ? and `tenant_id` = ? and `status` = 'APPLYING'"
	res, err := session.ExecCtx(ctx, query, reason, returnId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customReturnOrderModel) MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId int64, returnNo, refundNo string) error {
	query := "update `return_order` set `status` = 'REFUNDED', `refund_no` = ? where `return_no` = ? and `tenant_id` = ? and `status` = 'APPROVED'"
	res, err := session.ExecCtx(ctx, query, refundNo, returnNo, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customReturnOrderModel) ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*ReturnOrder, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`return_no` like ? or `order_no` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `return_order` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var list []*ReturnOrder
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+returnOrderRows+" from `return_order` where "+where+" order by `return_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// —— return_item ——

var _ ReturnItemModel = (*customReturnItemModel)(nil)

type (
	// ReturnItemModel 退货明细模型。
	ReturnItemModel interface {
		returnItemModel
		// InsertTx 事务内写明细。
		InsertTx(ctx context.Context, session sqlx.Session, data *ReturnItem) error
		// FindByReturn 退货明细列表。
		FindByReturn(ctx context.Context, tenantId, returnId int64) ([]*ReturnItem, error)
	}

	customReturnItemModel struct {
		*defaultReturnItemModel
		conn sqlx.SqlConn
	}
)

// NewReturnItemModel returns a model for the database table.
func NewReturnItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ReturnItemModel {
	return &customReturnItemModel{
		defaultReturnItemModel: newReturnItemModel(conn, c, opts...),
		conn:                   conn,
	}
}

func (m *customReturnItemModel) InsertTx(ctx context.Context, session sqlx.Session, data *ReturnItem) error {
	query := "insert into `return_item` (`item_id`, `return_id`, `sku_id`, `sn`, `qty`, `reason`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ItemId, data.ReturnId, data.SkuId, data.Sn,
		data.Qty, data.Reason, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customReturnItemModel) FindByReturn(ctx context.Context, tenantId, returnId int64) ([]*ReturnItem, error) {
	var list []*ReturnItem
	query := "select " + returnItemRows + " from `return_item` where `return_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, returnId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

// —— shipment_item ——

var _ ShipmentItemModel = (*customShipmentItemModel)(nil)

type (
	// ShipmentItemModel 发货单明细模型。
	ShipmentItemModel interface {
		shipmentItemModel
		// InsertTx 事务内写明细。
		InsertTx(ctx context.Context, session sqlx.Session, data *ShipmentItem) error
		// ListByShipment 发货明细。
		ListByShipment(ctx context.Context, tenantId, shipmentId int64) ([]*ShipmentItem, error)
	}

	customShipmentItemModel struct {
		*defaultShipmentItemModel
		conn sqlx.SqlConn
	}
)

// NewShipmentItemModel returns a model for the database table.
func NewShipmentItemModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ShipmentItemModel {
	return &customShipmentItemModel{
		defaultShipmentItemModel: newShipmentItemModel(conn, c, opts...),
		conn:                     conn,
	}
}

func (m *customShipmentItemModel) InsertTx(ctx context.Context, session sqlx.Session, data *ShipmentItem) error {
	query := "insert into `shipment_item` (`item_id`, `shipment_id`, `sku_id`, `sn`, `qty`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ItemId, data.ShipmentId, data.SkuId, data.Sn,
		data.Qty, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customShipmentItemModel) ListByShipment(ctx context.Context, tenantId, shipmentId int64) ([]*ShipmentItem, error) {
	var list []*ShipmentItem
	query := "select " + shipmentItemRows + " from `shipment_item` where `shipment_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `item_id`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, shipmentId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}
