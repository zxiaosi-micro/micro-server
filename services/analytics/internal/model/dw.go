// dw.go · 数仓星型模型 ETL（S10-02）：2 事实 + 3 维度 + 批次台账。
//
// 口径：平台级聚合面（tenant_id 是分析维度非隔离条件，model/registry.go 豁免登记）；
// INSERT..SELECT 跨库直读业务库（A1 应用账号有 micro_% 授权，对账 recon 同口径）。
// 幂等：维 INSERT IGNORE / ODKU 快照、事实 PK(单号) ODKU——重跑同值 affected=0（验收断言）。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DwEtlStat 单日 ETL 计量（affected 行数；重跑同值全 0）。
type DwEtlStat struct {
	DimDateRows   int64 // 日历维补行数
	DimTenantRows int64 // 租户维新增行数
	DimPartyRows  int64 // 参与方维快照刷新行数
	FactOrderRows int64 // 订单事实写入行数
	FactPayRows   int64 // 支付事实写入行数
}

// DwModel 数仓星型模型（2 事实 + 3 维度 + 批次台账）的 ETL 与调度对账面。
type DwModel interface {
	// EtlDay 单日 ETL（业务库 → dw 星型；ODKU/IGNORE 幂等）。
	EtlDay(ctx context.Context, day time.Time) (*DwEtlStat, error)
	// MissingDays 缺失批次查询（调度对账：etl_log 无行的日期）。
	MissingDays(ctx context.Context, from, to time.Time) ([]time.Time, error)
	// InsertEtlLog 批次台账落库（重跑覆盖计量，finished_at 随行刷新）。
	InsertEtlLog(ctx context.Context, day time.Time, stat *DwEtlStat, costMs int64) error
}

type dwModel struct{ conn sqlx.SqlConn }

// NewDwModel 创建数仓 ETL 模型（INSERT..SELECT 直读业务库，见文件头口径说明）。
func NewDwModel(conn sqlx.SqlConn) DwModel { return &dwModel{conn: conn} }

// EtlDay 单日 ETL：dim_date → dim_tenant → dim_party → fact_order → fact_payment。
func (m *dwModel) EtlDay(ctx context.Context, day time.Time) (*DwEtlStat, error) {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	stat := &DwEtlStat{}
	var err error

	// ① 日历维补行（INSERT IGNORE 幂等）
	q := "insert ignore into `dw_dim_date` (`date_key`,`year`,`quarter`,`month`,`day_of_month`,`week_of_year`,`weekday`,`is_weekend`) " +
		"values (?,?,?,?,?,?,?,?)"
	_, isoWeek := dayStart.ISOWeek()
	wd := int(dayStart.Weekday())
	if wd == 0 {
		wd = 7 // 1=周一…7=周日
	}
	if stat.DimDateRows, err = m.exec(ctx, q, dayStart, dayStart.Year(), (dayStart.Month()-1)/3+1,
		int(dayStart.Month()), dayStart.Day(), isoWeek, wd, b2i(wd >= 6)); err != nil {
		return nil, err
	}

	// ② 租户维补行（事实源 DISTINCT 提取；LEFT JOIN 目标表防 1093）
	q = "insert into `dw_dim_tenant` (`tenant_id`,`first_seen_at`) " +
		"select s.tenant_id, min(s.seen_at) from (" +
		"select o.tenant_id, min(o.created_at) as seen_at from micro_order.`order` o " +
		"where o.created_at >= ? and o.created_at < ? and o.deleted_at is null group by o.tenant_id " +
		"union all " +
		"select p.tenant_id, min(p.created_at) from micro_finance.payment p " +
		"where p.created_at >= ? and p.created_at < ? and p.deleted_at is null group by p.tenant_id) s " +
		"left join `dw_dim_tenant` t on t.tenant_id = s.tenant_id " +
		"where t.tenant_id is null group by s.tenant_id"
	if stat.DimTenantRows, err = m.exec(ctx, q, dayStart, dayEnd, dayStart, dayEnd); err != nil {
		return nil, err
	}

	// ③ 参与方维快照（ODKU 幂等刷新）
	q = "insert into `dw_dim_party` (`party_id`,`tenant_id`,`name`,`types`,`region`,`updated_at`) " +
		"select p.party_id, p.tenant_id, p.name, p.type, ifnull(p.region,''), p.updated_at " +
		"from micro_party.party p where p.updated_at >= ? and p.updated_at < ? and p.deleted_at is null " +
		"on duplicate key update `name`=values(`name`), `types`=values(`types`), " +
		"`region`=values(`region`), `updated_at`=values(`updated_at`)"
	if stat.DimPartyRows, err = m.exec(ctx, q, dayStart, dayEnd); err != nil {
		return nil, err
	}

	// ④ 订单事实（PK=order_no ODKU；date_key=下单日）
	q = "insert into `dw_fact_order` (`order_no`,`date_key`,`tenant_id`,`buyer_party_id`,`status`,`total_amount`,`source_created_at`,`source_updated_at`) " +
		"select o.order_no, date(o.created_at), o.tenant_id, ifnull(o.buyer_party_id,0), o.status, o.total_amount, o.created_at, o.updated_at " +
		"from micro_order.`order` o where o.created_at >= ? and o.created_at < ? and o.deleted_at is null " +
		"on duplicate key update `status`=values(`status`), `total_amount`=values(`total_amount`), " +
		"`source_updated_at`=values(`source_updated_at`)"
	if stat.FactOrderRows, err = m.exec(ctx, q, dayStart, dayEnd); err != nil {
		return nil, err
	}

	// ⑤ 支付事实（PK=payment_no ODKU；date_key=实收日；窗口含 paid_at——补抓历史单在当日支付）
	q = "insert into `dw_fact_payment` (`payment_no`,`date_key`,`tenant_id`,`order_no`,`channel`,`status`,`amount`,`paid_amount`,`paid_at`) " +
		"select p.payment_no, date(ifnull(p.paid_at, p.created_at)), p.tenant_id, p.order_no, p.channel, p.status, " +
		"p.amount, ifnull(p.paid_amount,0), p.paid_at " +
		"from micro_finance.payment p where ((p.created_at >= ? and p.created_at < ?) or (p.paid_at >= ? and p.paid_at < ?)) " +
		"and p.deleted_at is null " +
		"on duplicate key update `date_key`=values(`date_key`), `status`=values(`status`), " +
		"`paid_amount`=values(`paid_amount`), `paid_at`=values(`paid_at`)"
	if stat.FactPayRows, err = m.exec(ctx, q, dayStart, dayEnd, dayStart, dayEnd); err != nil {
		return nil, err
	}
	return stat, nil
}

// MissingDays 批次对账：[from,to] 内台账缺失的业务日（调度补跑依据）。
func (m *dwModel) MissingDays(ctx context.Context, from, to time.Time) ([]time.Time, error) {
	var days []string
	q := "select d.d from (with recursive seq as (select ? as d union all select d + interval 1 day from seq where d < ?) select d from seq) d " +
		"left join `dw_etl_log` l on l.etl_date = d.d where l.etl_date is null"
	if err := m.conn.QueryRowsCtx(ctx, &days, q, from.Format("2006-01-02"), to.Format("2006-01-02")); err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(days))
	for _, s := range days {
		t, err := time.ParseInLocation("2006-01-02", s, from.Location())
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// InsertEtlLog 批次台账（重跑覆盖；finished_at ON UPDATE 随行刷新）。
func (m *dwModel) InsertEtlLog(ctx context.Context, day time.Time, stat *DwEtlStat, costMs int64) error {
	q := "replace into `dw_etl_log` (`etl_date`,`dim_date_rows`,`dim_tenant_rows`,`dim_party_rows`,`fact_order_rows`,`fact_payment_rows`,`cost_ms`) " +
		"values (?,?,?,?,?,?,?)"
	_, err := m.conn.ExecCtx(ctx, q, day.Format("2006-01-02"),
		stat.DimDateRows, stat.DimTenantRows, stat.DimPartyRows, stat.FactOrderRows, stat.FactPayRows, costMs)
	return err
}

// exec 执行写 SQL 并返回 affected 行数（ETL 计量用）。
func (m *dwModel) exec(ctx context.Context, q string, args ...any) (int64, error) {
	res, err := m.conn.ExecCtx(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// b2i 布尔转 0/1（SQL 参数用）。
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
