// p4backfill · 读模型补投递（S8-04）：按业务日扫描业务库，重发缺失事件到 Kafka。
//
// 修复口径（E10 禁裸删）：差异只允许补投递重放——业务库是唯一事实源，
// 本工具把业务库里"当日应有的事件"重新发一遍（新 event_id、原 payload 语义），
// analytics 读模型按台账业务键幂等去重，缺的补上、已有的一笔不多。
// 反向（读模型多出）不删数据：差异留 recon_diff 留底，人工核查消费链路。
//
// 覆盖指标（report_metric_dict.recon=1 的 14 类事件中可从业务表重构的 13 类；
// stock.low 属状态边沿事件不可重构——对账不覆盖该口径，见字典）。
//
// 用法：
//
//	source tools/devenv.sh micro_analytics
//	go run ./tools/p4backfill -date 2026-10-11            # 全域补投递
//	go run ./tools/p4backfill -date 2026-10-11 -metric orders_created  # 单指标
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/segmentio/kafka-go"
)

var (
	dateStr = flag.String("date", "", "业务日 yyyy-MM-dd（必填）")
	metric  = flag.String("metric", "", "指标过滤（如 orders_created；空=全部）")
	dryRun  = flag.Bool("dry", false, "只统计不发 Kafka")
	brokers = envOr("KAFKA_BROKERS", "127.0.0.1:29092")
	dsn     = envOr("MICRO_DSN_ANALYTICS", "")
	verbose = flag.Bool("v", false, "逐条打印")
)

// backfillSource 单类事件的重构源（query 按业务日扫业务库 → 事件构造器）。
type backfillSource struct {
	metric  string // 对账指标（过滤键）
	eventTp string // 点号事件类型
	topic   string
	skip    bool // 同事件多指标口径（orders_paid=payments_count）不重复补发
	key     func(row map[string]any) string
	payload func(row map[string]any) (map[string]any, error)
	// occurred 列名（业务时刻，进信封 occurred_at——读模型按它取业务日）
	query string // 含 tenant_id + occurred 列扫描
}

var sources = []backfillSource{
	{
		metric: "orders_created", eventTp: "order.created", topic: "order_created",
		query: "SELECT order_no AS biz, tenant_id AS tid, created_at AS occurred, type AS order_type, total_amount AS total_amount, buyer_party_id AS buyer_party_id FROM micro_order.`order` WHERE created_at >= ? AND created_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["biz"]),
				"order_type": str(r["order_type"]), "total_amount": str(r["total_amount"]),
				"buyer_party_id": i64(r["buyer_party_id"]),
			}, nil
		},
	},
	{
		metric: "orders_paid", eventTp: "order.paid", topic: "order_paid",
		query: "SELECT order_no AS biz, tenant_id AS tid, paid_at AS occurred, payment_no AS payment_no, channel AS channel, paid_amount AS amount FROM micro_finance.payment WHERE status IN ('PAID','SETTLED') AND paid_at >= ? AND paid_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["biz"]),
				"payment_no": str(r["payment_no"]), "channel": str(r["channel"]),
				"paid_amount": str(r["amount"]), "paid_at": milli(r["occurred"]),
			}, nil
		},
	},
	{
		metric: "orders_cancelled", eventTp: "order.cancelled", topic: "order_cancelled",
		query: "SELECT order_no AS biz, tenant_id AS tid, updated_at AS occurred, cancel_reason AS reason FROM micro_order.`order` WHERE status = 'CANCELLED' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["biz"]),
				"reason": strOr(r["reason"], ""), "cancelled_by": "USER",
			}, nil
		},
	},
	{
		metric: "orders_pay_timeout", eventTp: "order.pay_timeout", topic: "order_pay_timeout",
		query: "SELECT order_no AS biz, tenant_id AS tid, updated_at AS occurred, pay_expire_at AS expire_at FROM micro_order.`order` WHERE status = 'PAY_TIMEOUT' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["biz"]),
				"expire_at": milli(r["expire_at"]),
			}, nil
		},
	},
	{
		metric: "returns_approved", eventTp: "order.return_approved", topic: "order_return_approved",
		query: "SELECT return_no AS biz, tenant_id AS tid, updated_at AS occurred, order_no AS order_no, refund_amount AS amount FROM micro_order.return_order WHERE status IN ('APPROVED','REFUNDED') AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "return_no": str(r["biz"]),
				"order_no": str(r["order_no"]), "refund_amount": str(r["amount"]), "reason": "backfill",
			}, nil
		},
	},
	{
		metric: "shipments_signed", eventTp: "shipment.signed", topic: "shipment_signed",
		// sns 不在 shipment 表（order_item 才有 SN）：devices_shipped 不参与对账，补投递 sns 置空
		query: "SELECT shipment_no AS biz, tenant_id AS tid, signed_at AS occurred, order_no AS order_no, signed_by AS signed_by FROM micro_order.shipment WHERE status = 'SIGNED' AND signed_at >= ? AND signed_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["order_no"]),
				"shipment_no": str(r["biz"]), "sns": []string{},
				"signed_by": strOr(r["signed_by"], "backfill"), "signed_at": milli(r["occurred"]),
			}, nil
		},
	},
	{
		metric: "stock_in_qty", eventTp: "stock.in", topic: "stock_in",
		query: "SELECT biz_no AS biz, tenant_id AS tid, created_at AS occurred, warehouse_id AS wid, sku_id AS sku, qty AS qty FROM micro_inventory.stock_record WHERE biz_type = 'STOCK_IN' AND created_at >= ? AND created_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "warehouse_id": i64(r["wid"]), "sku_id": i64(r["sku"]),
				"qty": i64(r["qty"]), "biz_no": str(r["biz"]), "sns": []string{},
			}, nil
		},
	},
	{
		metric: "stock_out_qty", eventTp: "stock.out", topic: "stock_out",
		query: "SELECT biz_no AS biz, tenant_id AS tid, created_at AS occurred, warehouse_id AS wid, sku_id AS sku, qty AS qty FROM micro_inventory.stock_record WHERE biz_type = 'DEDUCT' AND created_at >= ? AND created_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "warehouse_id": i64(r["wid"]), "sku_id": i64(r["sku"]),
				"qty": i64(r["qty"]), "biz_no": str(r["biz"]), "sns": []string{},
			}, nil
		},
	},
	{
		metric: "payments_count", eventTp: "order.paid", topic: "order_paid", // 与 orders_paid 同事件（finance 口径），跳过二次补发
		query: "", skip: true,
	},
	{
		metric: "refunds_count", eventTp: "payment.refunded", topic: "payment_refunded",
		query: "SELECT refund_no AS biz, tenant_id AS tid, updated_at AS occurred, order_no AS order_no, payment_no AS payment_no, amount AS amount, channel AS channel FROM micro_finance.refund WHERE status = 'SUCCESS' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "order_no": str(r["order_no"]),
				"payment_no": str(r["payment_no"]), "return_no": "", "refund_no": str(r["biz"]),
				"amount": str(r["amount"]), "channel": str(r["channel"]),
			}, nil
		},
	},
	{
		metric: "warranties_started", eventTp: "warranty.started", topic: "warranty_started",
		query: "SELECT warranty_no AS biz, tenant_id AS tid, start_at AS occurred, level AS level, target_key AS target_key, months AS months, end_at AS end_at FROM micro_contract.warranty WHERE start_at >= ? AND start_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "warranty_no": str(r["biz"]),
				"level": str(r["level"]), "target_key": str(r["target_key"]),
				"start_at": milli(r["occurred"]), "end_at": milli(r["end_at"]), "months": i64(r["months"]),
			}, nil
		},
	},
	{
		metric: "devices_activated", eventTp: "device.activated", topic: "device_activated",
		query: "SELECT sn AS biz, tenant_id AS tid, created_at AS occurred FROM micro_device.device_lifecycle_log WHERE event_type = 'device.activated' AND created_at >= ? AND created_at < ?",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "sn": str(r["biz"]), "activated_at": milli(r["occurred"]),
			}, nil
		},
	},
	{
		metric: "alerts_total", eventTp: "alert.created", topic: "alert_created",
		query: "SELECT alert_id AS biz, tenant_id AS tid, first_seen_at AS occurred, station_id AS station_id, level AS level, title AS title FROM micro_ops.alert WHERE first_seen_at >= ? AND first_seen_at < ?",
		key:   func(r map[string]any) string { return fmt.Sprintf("%d", i64(r["biz"])) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"alert_id": i64(r["biz"]), "tenant_id": i64(r["tid"]),
				"station_id": i64(r["station_id"]), "level": str(r["level"]),
				"source": "backfill", "metric": "backfill", "title": str(r["title"]),
				"first_seen_at": milli(r["occurred"]),
			}, nil
		},
	},
	{
		metric: "stations_created", eventTp: "station.created", topic: "station_created",
		query: "SELECT station_no AS biz, tenant_id AS tid, created_at AS occurred, station_id AS station_id, name AS name, order_no AS order_no FROM micro_station.station WHERE created_at >= ? AND created_at < ? AND deleted_at IS NULL",
		key:   func(r map[string]any) string { return str(r["biz"]) },
		payload: func(r map[string]any) (map[string]any, error) {
			return map[string]any{
				"tenant_id": i64(r["tid"]), "station_id": i64(r["station_id"]),
				"station_no": str(r["biz"]), "name": strOr(r["name"], ""), "order_no": strOr(r["order_no"], ""),
			}, nil
		},
	},
}

// skip 同事件多指标口径（orders_paid = payments_count）不重复补发——skip 源在 main 中直接跳过。

func main() {
	flag.Parse()
	if *dateStr == "" || dsn == "" {
		fmt.Fprintln(os.Stderr, "用法: p4backfill -date yyyy-MM-dd [-metric <code>] [-dry]  （需 MICRO_DSN_ANALYTICS，source tools/devenv.sh micro_analytics）")
		os.Exit(1)
	}
	day, err := time.ParseInLocation("2006-01-02", *dateStr, time.Local)
	must(err, "date 解析")
	dayEnd := day.AddDate(0, 0, 1)

	db, err := sql.Open("mysql", dsn)
	must(err, "业务库连接（MICRO_DSN_ANALYTICS 同实例跨库只读）")
	defer func() { _ = db.Close() }()

	w := &kafka.Writer{Addr: kafka.TCP(brokers), AllowAutoTopicCreation: false, Balancer: &kafka.Hash{}}
	defer func() { _ = w.Close() }()

	total := 0
	for _, src := range sources {
		if src.skip {
			continue
		}
		if *metric != "" && *metric != src.metric {
			continue
		}
		rows, err := queryRows(db, src.query, day, dayEnd)
		must(err, "扫描 "+src.metric)
		n := 0
		for _, row := range rows {
			payload, err := src.payload(row)
			if err != nil {
				fmt.Fprintf(os.Stderr, "payload 构造失败 %s: %v\n", src.metric, err)
				continue
			}
			env := map[string]any{
				"event_id":    newEventID(),
				"event_type":  src.eventTp,
				"tenant_id":   i64(row["tid"]),
				"trace_id":    "",
				"occurred_at": timeOf(row["occurred"]).Format(time.RFC3339Nano),
				"payload":     payload,
			}
			raw, _ := json.Marshal(env)
			if *verbose {
				fmt.Printf("  %s key=%s payload=%s\n", src.topic, src.key(row), string(raw))
			}
			if !*dryRun {
				err := w.WriteMessages(context.Background(), kafka.Message{
					Topic: src.topic, Key: []byte(src.key(row)), Value: raw,
				})
				must(err, "Kafka 生产 "+src.topic)
			}
			n++
		}
		if n > 0 {
			fmt.Printf("  %-20s %s: %d 条\n", src.metric, src.eventTp, n)
		}
		total += n
	}
	action := "补投递"
	if *dryRun {
		action = "dry-run 统计"
	}
	fmt.Printf("== p4backfill 完成（%s date=%s）：共 %d 条 ==\n", action, *dateStr, total)
}

// ---- helpers ----

func queryRows(db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	if query == "" {
		return nil, nil
	}
	rs, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			row[c] = vals[i]
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}

func strOr(v any, def string) string {
	if s := str(v); s != "" {
		return s
	}
	return def
}

func i64(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case int64:
		return t
	case []byte:
		var n int64
		_, _ = fmt.Sscanf(string(t), "%d", &n)
		return n
	default:
		return 0
	}
}

func timeOf(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	case []byte:
		tm, _ := time.ParseInLocation("2006-01-02 15:04:05.999", string(t), time.Local)
		return tm
	default:
		return time.Now()
	}
}

func milli(v any) int64 { return timeOf(v).UnixMilli() }

// newEventID 补投递事件用新 ID（128bit hex，与 eventbus.NewEventID 同形）。
func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "p4backfill FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
