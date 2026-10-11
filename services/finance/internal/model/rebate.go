// rebate_settlement custom 覆写（ADR-08）：租户过滤显式携带 + PENDING 守卫 Upsert（S10-03 经销商返利）。

package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RebateSettlementModel = (*customRebateSettlementModel)(nil)

type (
	// RebateSettlementModel 返利结算 custom 接口。
	RebateSettlementModel interface {
		rebateSettlementModel
		// UpsertTx 事务内插入（uk tenant+period+party；金额仅 PENDING 可覆盖——CONFIRMED/PAID 业务锁）。
		UpsertTx(ctx context.Context, session sqlx.Session, data *RebateSettlement) error
		// FindOneScoped 租户内主键查询（软删过滤）。
		FindOneScoped(ctx context.Context, tenantId, settlementId int64) (*RebateSettlement, error)
		// ListByPeriod 周期内全部结算单（对账扫描）。
		ListByPeriod(ctx context.Context, tenantId int64, period string) ([]*RebateSettlement, error)
		// SumPaidAmount 周期内实付计提基数（payer=经销商，PAID/SETTLED）。
		SumPaidAmount(ctx context.Context, tenantId, partyId int64, periodStart, periodEnd string) (string, int64, error)
		// CasStatus 状态机推进 PENDING→CONFIRMED→PAID（affected==0=状态冲突）。
		CasStatus(ctx context.Context, tenantId, settlementId int64, from, to string) (int64, error)
		// ListPage 结算列表。
		ListPage(ctx context.Context, tenantId int64, period, status string, partyId, limit, offset int64) ([]*RebateSettlement, int64, error)
	}

	customRebateSettlementModel struct {
		*defaultRebateSettlementModel
		conn sqlx.SqlConn
	}
)

// NewRebateSettlementModel returns a model for the database table.
func NewRebateSettlementModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RebateSettlementModel {
	return &customRebateSettlementModel{
		defaultRebateSettlementModel: newRebateSettlementModel(conn, c, opts...),
		conn:                         conn,
	}
}

const rebateSoftFilter = "`deleted_at` is null"

func (m *customRebateSettlementModel) UpsertTx(ctx context.Context, session sqlx.Session, data *RebateSettlement) error {
	query := "insert into `rebate_settlement` (`settlement_id`,`period`,`party_id`,`rule_snapshot`,`amount`,`status`,`tenant_id`,`created_by`,`updated_by`) values (?,?,?,?,?,?,?,?,?) " +
		"on duplicate key update `amount`=if(`status`='PENDING', values(`amount`), `amount`), " +
		"`rule_snapshot`=if(`status`='PENDING', values(`rule_snapshot`), `rule_snapshot`), `updated_by`=values(`updated_by`)"
	_, err := session.ExecCtx(ctx, query,
		data.SettlementId, data.Period, data.PartyId, data.RuleSnapshot, data.Amount,
		data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customRebateSettlementModel) FindOneScoped(ctx context.Context, tenantId, settlementId int64) (*RebateSettlement, error) {
	var res RebateSettlement
	query := "select " + rebateSettlementRows + " from `rebate_settlement` where `settlement_id` = ? and `tenant_id` = ? and " + rebateSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, settlementId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customRebateSettlementModel) ListByPeriod(ctx context.Context, tenantId int64, period string) ([]*RebateSettlement, error) {
	var list []*RebateSettlement
	query := "select " + rebateSettlementRows + " from `rebate_settlement` where `tenant_id` = ? and `period` = ? and " + rebateSoftFilter + " order by `party_id`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, period); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customRebateSettlementModel) SumPaidAmount(ctx context.Context, tenantId, partyId int64, periodStart, periodEnd string) (string, int64, error) {
	var row struct {
		Amount string `db:"amount"`
		Cnt    int64  `db:"cnt"`
	}
	query := "select ifnull(sum(`amount`), 0) as `amount`, count(*) as `cnt` from `payment` where `tenant_id` = ? and `payer_party_id` = ? " +
		"and `status` in ('PAID','SETTLED') and `paid_at` >= ? and `paid_at` < ? and " + rebateSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &row, query, tenantId, partyId, periodStart, periodEnd); err != nil {
		return "", 0, err
	}
	return row.Amount, row.Cnt, nil
}

func (m *customRebateSettlementModel) CasStatus(ctx context.Context, tenantId, settlementId int64, from, to string) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"update `rebate_settlement` set `status` = ? where `settlement_id` = ? and `tenant_id` = ? and `status` = ? and "+rebateSoftFilter,
		to, settlementId, tenantId, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customRebateSettlementModel) ListPage(ctx context.Context, tenantId int64, period, status string, partyId, limit, offset int64) ([]*RebateSettlement, int64, error) {
	where := "`tenant_id` = ? and " + rebateSoftFilter
	args := []any{tenantId}
	if period != "" {
		where += " and `period` = ?"
		args = append(args, period)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if partyId > 0 {
		where += " and `party_id` = ?"
		args = append(args, partyId)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"select count(*) from `rebate_settlement` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*RebateSettlement
	query := "select " + rebateSettlementRows + " from `rebate_settlement` where " + where + " order by `period` desc, `party_id` limit ? offset ?"
	args = append(args, limit, offset)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
