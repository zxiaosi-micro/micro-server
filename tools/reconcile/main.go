// reconcile · 对账兜底（S5-05，E10）：库存日结 + 资金日结 → 差异报告。
//
// 修复纪律（E10，02 §9.3）：差异修复只允许补投递重放（-fix 将 event_dead/event_retry 中
// 可重放事件重新入列），禁止裸删/直改业务数据。
//
// 用法：
//	go run ./tools/reconcile                     # 今日报告（stdout）
//	go run ./tools/reconcile -date 2026-10-08    # 指定日
//	go run ./tools/reconcile -fix                # 差异修复 = 补投递重放（重试事件提前到期 + 死信重新入列）
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"micro-server/tools/internal/devcfg"
)

type stockDiff struct {
	TenantID   int64   `json:"tenant_id"`
	Warehouse  int64   `json:"warehouse_id"`
	SkuID      int64   `json:"sku_id"`
	DbAvail    int64   `json:"db_available"`
	DbLocked   int64   `json:"db_locked"`
	ReplayAvail int64  `json:"replay_available"`
	ReplayLocked int64 `json:"replay_locked"`
}

type fundRow struct {
	Channel string  `json:"channel"`
	Count   int64   `json:"count"`
	Total   float64 `json:"total"`
}

func main() {
	date := flag.String("date", time.Now().Format("2006-01-02"), "业务日期 yyyy-MM-dd")
	fix := flag.Bool("fix", false, "差异修复 = 补投递重放（E10，唯一允许的修复）")
	flag.Parse()

	dsn := devcfg.Get("MICRO_DEV_MYSQL_APP_DSN", fmt.Sprintf(
		"%s:%s@tcp(%s)/?charset=utf8mb4&parseTime=true&loc=Local&timeout=5s",
		devcfg.Get("MICRO_DEV_MYSQL_APP_USER", "micro_app"),
		devcfg.Get("MICRO_DEV_MYSQL_APP_PW", ""),
		devcfg.Get("MICRO_DEV_MYSQL_HOST", "127.0.0.1:23306")))
	conn := sqlx.NewMysql(dsn)
	ctx := context.Background()

	day, err := time.ParseInLocation("2006-01-02", *date, time.Local)
	must(err)
	from, to := day, day.AddDate(0, 0, 1)

	report := map[string]any{"date": *date, "generated_at": time.Now().Format(time.RFC3339)}

	// ---- 库存日结：inventory 账面 vs stock_record 重放汇总 ----
	stockDiffs := stockReconcile(ctx, conn)
	report["stock_diffs"] = stockDiffs
	report["stock_diff_count"] = len(stockDiffs)

	// ---- 资金日结：payment 渠道汇总 + refund 汇总 ----
	var payments []fundRow
	must(conn.QueryRowsCtx(ctx, &payments,
		"select `channel`, count(*) as `cnt`, ifnull(sum(`paid_amount`),0) as `total` from micro_finance.`payment` where `status` in ('PAID','SETTLED') and `paid_at` >= ? and `paid_at` < ? and `deleted_at` is null group by `channel`", from, to))
	var refund struct {
		Total float64 `db:"total"`
		Cnt   int64   `db:"cnt"`
	}
	must(conn.QueryRowCtx(ctx, &refund,
		"select ifnull(sum(`amount`),0) as `total`, count(*) as `cnt` from micro_finance.`refund` where `status` = 'SUCCESS' and `created_at` >= ? and `created_at` < ? and `deleted_at` is null", from, to))
	report["payments"] = payments
	report["refunds"] = refund

	// ---- Outbox 积压 / 重试 / 死信 ----
	var backlog struct {
		Pending int64 `db:"pending"`
		Retry   int64 `db:"retry"`
		Dead    int64 `db:"dead"`
	}
	must(conn.QueryRowCtx(ctx, &backlog,
		"select (select count(*) from micro_order.`event_outbox` where `status` in ('PENDING','RETRY')) as `pending`, "+
			"(select count(*) from micro_order.`event_retry`) as `retry`, "+
			"(select count(*) from micro_order.`event_dead`) as `dead`"))
	var backlogF struct {
		Pending int64 `db:"pending"`
		Retry   int64 `db:"retry"`
		Dead    int64 `db:"dead"`
	}
	must(conn.QueryRowCtx(ctx, &backlogF,
		"select (select count(*) from micro_finance.`event_outbox` where `status` in ('PENDING','RETRY')) as `pending`, "+
			"(select count(*) from micro_finance.`event_retry`) as `retry`, "+
			"(select count(*) from micro_finance.`event_dead`) as `dead`"))
	report["order_event_backlog"] = backlog
	report["finance_event_backlog"] = backlogF

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	if len(stockDiffs) > 0 {
		fmt.Printf("reconcile: 库存差异 %d 处 —— 修复只允许补投递重放（-fix），禁止裸改（E10）\n", len(stockDiffs))
	}
	if *fix {
		must(replayFix(ctx, conn))
		fmt.Println("reconcile: 补投递重放完成（retry 提前到期 + dead 重新入列；三次服务内 Relay/Subscriber 接续）")
	}
}

// stockReconcile 库存账面 vs 流水重放：available = stock_in 累计 - deduct - spare_out + spare_return + return_in，
// locked = reserve - release - deduct（简化口径：以 stock_record biz_type 汇总）。
func stockReconcile(ctx context.Context, conn sqlx.SqlConn) []stockDiff {
	var diffs []stockDiff
	var rows []struct {
		TenantID  int64 `db:"tenant_id"`
		Warehouse int64 `db:"warehouse_id"`
		SkuID     int64 `db:"sku_id"`
		Avail     int64 `db:"avail"`
		Locked    int64 `db:"locked"`
	}
	must(conn.QueryRowsCtx(ctx, &rows,
		"select `tenant_id`, `warehouse_id`, `sku_id`, `available` as `avail`, `locked` as `locked` from micro_inventory.`inventory` where `deleted_at` is null"))
	for _, r := range rows {
		var replay struct {
			Avail  sql.NullFloat64 `db:"avail"`
			Locked sql.NullFloat64 `db:"locked"`
		}
		qerr := conn.QueryRowCtx(ctx, &replay,
			"select sum(case when `biz_type` in ('STOCK_IN','SPARE_RETURN','RETURN_IN','TRANSFER_IN') then `qty` "+
				"when `biz_type` in ('DEDUCT','SPARE_OUT') then -`qty` else 0 end) as `avail`, "+
				"sum(case when `biz_type` = 'RESERVE' then `qty` when `biz_type` in ('RELEASE','DEDUCT') then -`qty` else 0 end) as `locked` "+
				"from micro_inventory.`stock_record` where `tenant_id` = ? and `warehouse_id` = ? and `sku_id` = ?",
			r.TenantID, r.Warehouse, r.SkuID)
		if qerr != nil {
			continue
		}
		ra, rl := int64(replay.Avail.Float64), int64(replay.Locked.Float64)
		if ra != r.Avail || rl != r.Locked {
			diffs = append(diffs, stockDiff{
				TenantID: r.TenantID, Warehouse: r.Warehouse, SkuID: r.SkuID,
				DbAvail: r.Avail, DbLocked: r.Locked, ReplayAvail: ra, ReplayLocked: rl,
			})
		}
	}
	return diffs
}

// replayFix 补投递重放（E10）：event_retry 的 next_retry_at 提前到 now；event_dead 重新写回 retry 表。
// 不改业务数据、不删事件行——重放语义完整保留。
func replayFix(ctx context.Context, conn sqlx.SqlConn) error {
	for _, db := range []string{"micro_order", "micro_finance", "micro_contract"} {
		if _, err := conn.ExecCtx(ctx,
			"update "+db+".`event_retry` set `next_retry_at` = now(3), `status` = 'RETRY' where `status` = 'RETRY'"); err != nil {
			return err
		}
		// 死信 → 重投行（重试计数清零重新退避；死信行保留留痕）
		if _, err := conn.ExecCtx(ctx,
			"insert ignore into "+db+".`event_retry` (`event_id`, `consumer_group`, `event_type`, `topic`, `partition_key`, `tenant_id`, `trace_id`, `payload`, `status`, `retry_count`, `next_retry_at`, `last_error`) "+
				"select `event_id`, ifnull(`consumer_group`,'reconcile-replay'), ifnull(`event_type`,''), ifnull(`topic`,''), '', `tenant_id`, ifnull(`trace_id`,''), `payload`, 'RETRY', 0, now(3), 'reconcile replay' "+
				"from "+db+".`event_dead` where `source` = 'consume'"); err != nil {
			return err
		}
	}
	return nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "reconcile:", err)
		os.Exit(1)
	}
}
