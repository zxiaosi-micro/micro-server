// finance_db 各表 custom 覆写（ADR-08）：租户过滤显式携带 + 事务方法 + CAS 状态机。
// payment/refund 两个核心写面；invoice/reconcile_task/rebate_settlement 见各自 custom 文件。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— payment ——

var _ PaymentModel = (*customPaymentModel)(nil)

type (
	// PaymentModel 支付单模型（payment_no UK + channel_txn_id UK）。
	PaymentModel interface {
		paymentModel
		// InsertTx 事务内建支付单。
		InsertTx(ctx context.Context, session sqlx.Session, data *Payment) error
		// FindOneByNo 租户过滤按单号取。
		FindOneByNo(ctx context.Context, tenantId int64, paymentNo string) (*Payment, error)
		// FindOneByNoForUpdateTx 行锁取（确认/复核并发入口）。
		FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, paymentNo string) (*Payment, error)
		// FindActiveByOrder 单订单活跃支付单（PAYING/REVIEWING；PayOrder 幂等锚点）。
		FindActiveByOrder(ctx context.Context, tenantId int64, orderNo string) (*Payment, error)
		// CASStatusTx 状态机 CAS（PAYING/REVIEWING/PAID/REJECTED/SETTLED/CLOSED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, from []string, to string) error
		// MarkPaidTx 支付确认回写（渠道流水 + 实收金额 + paid_at）。
		MarkPaidTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, txnId, paidAmount string, paidAt time.Time) error
		// MarkReviewedTx 复核回写（第二人 + 意见；approve=false → REJECTED）。
		MarkReviewedTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, approvedBy int64, approve bool, remark string) error
		// ListPage 支付列表（keyword=payment_no/order_no；REVIEWING 高亮）。
		ListPage(ctx context.Context, tenantId int64, keyword, channel, status string, page, size int64) ([]*Payment, int64, error)
		// SumByChannelDate 渠道日结（tools/reconcile 资金日结数据源）。
		SumByChannelDate(ctx context.Context, from, to time.Time) ([]*ChannelSumRow, error)
	}

	// ChannelSumRow 渠道日结行（对账用）。
	ChannelSumRow struct {
		Channel     string  `db:"channel"`
		Cnt         int64   `db:"cnt"`
		TotalAmount float64 `db:"total"`
	}

	customPaymentModel struct {
		*defaultPaymentModel
		conn sqlx.SqlConn
	}
)

// NewPaymentModel returns a model for the database table.
func NewPaymentModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PaymentModel {
	return &customPaymentModel{
		defaultPaymentModel: newPaymentModel(conn, c, opts...),
		conn:                conn,
	}
}

func (m *customPaymentModel) InsertTx(ctx context.Context, session sqlx.Session, data *Payment) error {
	query := "insert into `payment` (`payment_id`, `payment_no`, `order_no`, `channel`, `status`, `amount`, `payer_party_id`, `created_by`, `pay_params`, `remark`, `tenant_id`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.PaymentId, data.PaymentNo, data.OrderNo, data.Channel,
		data.Status, data.Amount, data.PayerPartyId, data.CreatedBy, data.PayParams, data.Remark, data.TenantId)
	return err
}

func (m *customPaymentModel) FindOneByNo(ctx context.Context, tenantId int64, paymentNo string) (*Payment, error) {
	var res Payment
	query := "select " + paymentRows + " from `payment` where `payment_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, paymentNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customPaymentModel) FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, paymentNo string) (*Payment, error) {
	var res Payment
	query := "select " + paymentRows + " from `payment` where `payment_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update"
	if err := session.QueryRowCtx(ctx, &res, query, paymentNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customPaymentModel) FindActiveByOrder(ctx context.Context, tenantId int64, orderNo string) (*Payment, error) {
	var res Payment
	query := "select " + paymentRows + " from `payment` where `order_no` = ? and `tenant_id` = ? and `status` in ('PAYING','REVIEWING') and `deleted_at` is null order by `payment_id` desc limit 1"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customPaymentModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, paymentId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `payment` set `status` = ? where `payment_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customPaymentModel) MarkPaidTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, txnId, paidAmount string, paidAt time.Time) error {
	query := "update `payment` set `status` = 'PAID', `channel_txn_id` = ?, `paid_amount` = ?, `paid_at` = ? where `payment_id` = ? and `tenant_id` = ? and `status` in ('PAYING','REVIEWING')"
	res, err := session.ExecCtx(ctx, query, txnId, paidAmount, paidAt, paymentId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customPaymentModel) MarkReviewedTx(ctx context.Context, session sqlx.Session, tenantId, paymentId int64, approvedBy int64, approve bool, remark string) error {
	status := "PAID"
	if !approve {
		status = "REJECTED"
	}
	query := "update `payment` set `status` = ?, `approved_by` = ?, `approve_remark` = ?, `paid_at` = if(? = 'PAID', now(3), `paid_at`) where `payment_id` = ? and `tenant_id` = ? and `status` = 'REVIEWING'"
	res, err := session.ExecCtx(ctx, query, status, approvedBy, remark, status, paymentId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customPaymentModel) ListPage(ctx context.Context, tenantId int64, keyword, channel, status string, page, size int64) ([]*Payment, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`payment_no` like ? or `order_no` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if channel != "" {
		where += " and `channel` = ?"
		args = append(args, channel)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `payment` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var list []*Payment
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+paymentRows+" from `payment` where "+where+" order by `payment_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customPaymentModel) SumByChannelDate(ctx context.Context, from, to time.Time) ([]*ChannelSumRow, error) {
	var rows []*ChannelSumRow
	query := "select `channel`, count(*) as `cnt`, ifnull(sum(`paid_amount`),0) as `total` from `payment` where `status` in ('PAID','SETTLED') and `paid_at` >= ? and `paid_at` < ? and `deleted_at` is null group by `channel`"
	if err := m.QueryRowsNoCacheCtx(ctx, &rows, query, from, to); err != nil {
		return nil, err
	}
	return rows, nil
}

// —— refund ——

var _ RefundModel = (*customRefundModel)(nil)

type (
	// RefundModel 退款单模型（幂等键 payment_no+return_no UK）。
	RefundModel interface {
		refundModel
		// InsertTx 事务内建退款单（1062 → 幂等重放）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Refund) error
		// FindOneByNo 租户过滤按单号取。
		FindOneByNo(ctx context.Context, tenantId int64, refundNo string) (*Refund, error)
		// MarkSuccessTx 退款成功回写（dev mock 即时；真实渠道回调后）。
		MarkSuccessTx(ctx context.Context, session sqlx.Session, tenantId int64, refundNo, channelRefundId string) error
		// ListPage 退款列表。
		ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*Refund, int64, error)
		// SumByDate 退款日结（对账）。
		SumByDate(ctx context.Context, from, to time.Time) (float64, int64, error)
	}

	customRefundModel struct {
		*defaultRefundModel
		conn sqlx.SqlConn
	}
)

// NewRefundModel returns a model for the database table.
func NewRefundModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RefundModel {
	return &customRefundModel{
		defaultRefundModel: newRefundModel(conn, c, opts...),
		conn:               conn,
	}
}

func (m *customRefundModel) InsertTx(ctx context.Context, session sqlx.Session, data *Refund) error {
	query := "insert into `refund` (`refund_id`, `refund_no`, `payment_no`, `order_no`, `return_no`, `amount`, `channel`, `status`, `reason`, `tenant_id`, `created_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.RefundId, data.RefundNo, data.PaymentNo, data.OrderNo,
		data.ReturnNo, data.Amount, data.Channel, data.Status, data.Reason, data.TenantId, data.CreatedBy)
	return err
}

func (m *customRefundModel) FindOneByNo(ctx context.Context, tenantId int64, refundNo string) (*Refund, error) {
	var res Refund
	query := "select " + refundRows + " from `refund` where `refund_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, refundNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customRefundModel) MarkSuccessTx(ctx context.Context, session sqlx.Session, tenantId int64, refundNo, channelRefundId string) error {
	query := "update `refund` set `status` = 'SUCCESS', `channel_refund_id` = ? where `refund_no` = ? and `tenant_id` = ? and `status` = 'PROCESSING'"
	_, err := session.ExecCtx(ctx, query, channelRefundId, refundNo, tenantId)
	return err
}

func (m *customRefundModel) ListPage(ctx context.Context, tenantId int64, keyword, status string, page, size int64) ([]*Refund, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`refund_no` like ? or `payment_no` like ? or `order_no` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `refund` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Refund
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+refundRows+" from `refund` where "+where+" order by `refund_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customRefundModel) SumByDate(ctx context.Context, from, to time.Time) (float64, int64, error) {
	var row struct {
		Total float64 `db:"total"`
		Cnt   int64   `db:"cnt"`
	}
	query := "select ifnull(sum(`amount`),0) as `total`, count(*) as `cnt` from `refund` where `status` = 'SUCCESS' and `created_at` >= ? and `created_at` < ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &row, query, from, to); err != nil {
		return 0, 0, err
	}
	return row.Total, row.Cnt, nil
}
