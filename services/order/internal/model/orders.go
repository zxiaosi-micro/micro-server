// order_db 各表 custom 覆写（ADR-08）：租户过滤显式携带 + 事务内方法 + 扫描查询。
// 约定：写路径一律 Tx 后缀（session 必传，nil 会 panic——调用方用 TransactCtx 组装）；
// 读路径 NoCache（租户敏感查询不走主键行缓存，02 §6.2 约定 2）；扫描查询供 cron 推进器用（E16）。

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)
// —— order ——

var _ OrderModel = (*customOrderModel)(nil)

type (
	// OrderModel 订单模型。
	OrderModel interface {
		orderModel
		// InsertTx 事务内建单（Saga 步骤 1 本地事务）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Order) error
		// FindOneScoped 租户过滤取单。
		FindOneScoped(ctx context.Context, tenantId, orderId int64) (*Order, error)
		// FindOneByNo 按单号取（租户过滤）。
		FindOneByNo(ctx context.Context, tenantId int64, orderNo string) (*Order, error)
		// FindOneByNoForUpdateTx 事务内行锁取单（Saga 推进/取消并发入口）。
		FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, orderNo string) (*Order, error)
		// CASStatus 订单状态机 CAS 转移（from 允许多态，逗号分隔；affected=0 即状态已漂移）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64, from []string, to string) error
		// MarkPaidTx 回写支付完成（order_paid 事件驱动）。
		MarkPaidTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64, paidAt time.Time) error
		// ListPage 订单列表（keyword/type/status 过滤，新→旧）。
		ListPage(ctx context.Context, tenantId int64, keyword, orderType, status string, page, size int64) ([]*Order, int64, error)
		// FindPayExpired 支付超时扫描（status 仍在支付窗口且到期；后台作业无租户上下文）。
		FindPayExpired(ctx context.Context, now time.Time, limit int) ([]*Order, error)
		// SumPaidSales 已支付销售汇总（PAID 起含，FR 统计口径）。
		SumPaidSales(ctx context.Context, tenantId int64, from, to time.Time) (float64, int64, error)
	}

	customOrderModel struct {
		*defaultOrderModel
		conn sqlx.SqlConn
	}
)

// NewOrderModel returns a model for the database table.
func NewOrderModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrderModel {
	return &customOrderModel{
		defaultOrderModel: newOrderModel(conn, c, opts...),
		conn:              conn,
	}
}

func (m *customOrderModel) InsertTx(ctx context.Context, session sqlx.Session, data *Order) error {
	query := "insert into `order` (`order_id`, `order_no`, `type`, `status`, `buyer_party_id`, `total_amount`, `pay_expire_at`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.OrderId, data.OrderNo, data.Type, data.Status,
		data.BuyerPartyId, data.TotalAmount, data.PayExpireAt, data.Remark,
		data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customOrderModel) FindOneScoped(ctx context.Context, tenantId, orderId int64) (*Order, error) {
	var res Order
	query := "select " + orderRows + " from `order` where `order_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, orderId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customOrderModel) FindOneByNo(ctx context.Context, tenantId int64, orderNo string) (*Order, error) {
	var res Order
	query := "select " + orderRows + " from `order` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customOrderModel) FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, orderNo string) (*Order, error) {
	var res Order
	query := "select " + orderRows + " from `order` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update"
	if err := session.QueryRowCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customOrderModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, orderId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := fmt.Sprintf("update `order` set `status` = ? where `order_id` = ? and `tenant_id` = ? and `status` in (%s)", placeholders)
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customOrderModel) MarkPaidTx(ctx context.Context, session sqlx.Session, tenantId, orderId int64, paidAt time.Time) error {
	query := "update `order` set `paid_at` = ? where `order_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, paidAt, orderId, tenantId)
	return err
}

func (m *customOrderModel) ListPage(ctx context.Context, tenantId int64, keyword, orderType, status string, page, size int64) ([]*Order, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and `order_no` like ?"
		args = append(args, "%"+keyword+"%")
	}
	if orderType != "" {
		where += " and `type` = ?"
		args = append(args, orderType)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, fmt.Sprintf("select count(*) from `order` where %s", where), args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	var list []*Order
	listQuery := fmt.Sprintf("select %s from `order` where %s order by `order_id` desc limit ? offset ?", orderRows, where)
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customOrderModel) FindPayExpired(ctx context.Context, now time.Time, limit int) ([]*Order, error) {
	var list []*Order
	query := "select " + orderRows + " from `order` where `pay_expire_at` is not null and `pay_expire_at` <= ? and `status` in ('CREATED','LOCKED','PAYING') and `deleted_at` is null limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, now, limit); err != nil {
		// QueryRowCtx 对多行结果同样适用（scan 进 slice）；ErrNotFound 视为空集
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return list, nil
}

func (m *customOrderModel) SumPaidSales(ctx context.Context, tenantId int64, from, to time.Time) (float64, int64, error) {
	var row struct {
		Total float64 `db:"total"`
		Cnt   int64   `db:"cnt"`
	}
	query := "select ifnull(sum(`total_amount`),0) as `total`, count(*) as `cnt` from `order` where `tenant_id` = ? and `paid_at` >= ? and `paid_at` < ? and `status` in ('PAID','STOCK_OUT','CONTRACTED','DONE') and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &row, query, tenantId, from, to); err != nil {
		if err == ErrNotFound {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	return row.Total, row.Cnt, nil
}
