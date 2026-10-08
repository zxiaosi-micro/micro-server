// invoice / reconcile_task / rebate_settlement 表 custom 覆写（ADR-08）。

package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— invoice ——

var _ InvoiceModel = (*customInvoiceModel)(nil)

type (
	// InvoiceModel 发票模型（ISSUED/REVERSED 状态机桩）。
	InvoiceModel interface {
		invoiceModel
		// InsertTx 开票落库。
		InsertTx(ctx context.Context, data *Invoice) error
		// FindOneByNo 租户过滤按发票号取。
		FindOneByNo(ctx context.Context, tenantId int64, invoiceNo string) (*Invoice, error)
		// ListPage 发票列表。
		ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*Invoice, int64, error)
	}

	customInvoiceModel struct {
		*defaultInvoiceModel
		conn sqlx.SqlConn
	}
)

// NewInvoiceModel returns a model for the database table.
func NewInvoiceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) InvoiceModel {
	return &customInvoiceModel{
		defaultInvoiceModel: newInvoiceModel(conn, c, opts...),
		conn:                conn,
	}
}

func (m *customInvoiceModel) InsertTx(ctx context.Context, data *Invoice) error {
	query := "insert into `invoice` (`invoice_id`, `invoice_no`, `payment_no`, `order_no`, `title`, `tax_no`, `amount`, `status`, `tenant_id`, `created_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := m.conn.ExecCtx(ctx, query, data.InvoiceId, data.InvoiceNo, data.PaymentNo, data.OrderNo,
		data.Title, data.TaxNo, data.Amount, data.Status, data.TenantId, data.CreatedBy)
	return err
}

func (m *customInvoiceModel) FindOneByNo(ctx context.Context, tenantId int64, invoiceNo string) (*Invoice, error) {
	var res Invoice
	query := "select " + invoiceRows + " from `invoice` where `invoice_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, invoiceNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInvoiceModel) ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*Invoice, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`invoice_no` like ? or `payment_no` like ? or `order_no` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `invoice` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Invoice
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+invoiceRows+" from `invoice` where "+where+" order by `invoice_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// —— reconcile_task ——

var _ ReconcileTaskModel = (*customReconcileTaskModel)(nil)

type (
	// ReconcileTaskModel 对账任务模型。
	ReconcileTaskModel interface {
		reconcileTaskModel
		// InsertTx 任务落库（同 type+biz_date UK 幂等）。
		InsertTx(ctx context.Context, data *ReconcileTask) error
		// FindOneScoped 租户过滤取任务。
		FindOneScoped(ctx context.Context, tenantId, taskId int64) (*ReconcileTask, error)
		// ListPage 任务列表（OPEN 优先）。
		ListPage(ctx context.Context, tenantId int64, status string, page, size int64) ([]*ReconcileTask, int64, error)
	}

	customReconcileTaskModel struct {
		*defaultReconcileTaskModel
		conn sqlx.SqlConn
	}
)

// NewReconcileTaskModel returns a model for the database table.
func NewReconcileTaskModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ReconcileTaskModel {
	return &customReconcileTaskModel{
		defaultReconcileTaskModel: newReconcileTaskModel(conn, c, opts...),
		conn:                      conn,
	}
}

func (m *customReconcileTaskModel) InsertTx(ctx context.Context, data *ReconcileTask) error {
	query := "insert into `reconcile_task` (`task_id`, `task_no`, `type`, `biz_date`, `diff_report`, `status`, `tenant_id`, `created_by`) values (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := m.conn.ExecCtx(ctx, query, data.TaskId, data.TaskNo, data.Type, data.BizDate,
		data.DiffReport, data.Status, data.TenantId, data.CreatedBy)
	return err
}

func (m *customReconcileTaskModel) FindOneScoped(ctx context.Context, tenantId, taskId int64) (*ReconcileTask, error) {
	var res ReconcileTask
	query := "select " + reconcileTaskRows + " from `reconcile_task` where `task_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, taskId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customReconcileTaskModel) ListPage(ctx context.Context, tenantId int64, status string, page, size int64) ([]*ReconcileTask, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `reconcile_task` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*ReconcileTask
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+reconcileTaskRows+" from `reconcile_task` where "+where+" order by `task_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
