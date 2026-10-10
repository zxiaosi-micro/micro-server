// p4smoke · 分析域全链冒烟（S8 阶段验收：读模型/指标字典/对账抓漏发/补投递闭环/异步导出）。
//
// 链路：双侧种子（业务库直插 + Kafka 信封事件）→ analytics 读模型（台账幂等 + 六大日聚合）
// → GetReport 断言 → ListMetricDict（≥18）→ RunRecon 抓"注入漏发事件"（orders_created 差异）
// → 差异站内信（micro_notification.message）→ p4backfill 补投递 → 复跑对账差异率 0
// → RequestExport CSV/XLSX/PDF → DONE + file 归档。
//
// 前置：compose core+edge、mqinit、analytics/file/notification 进程运行中；
//
//	RECON_DIFF 通知模板由本工具幂等种子（micro_notification）。
//
// 纪律：严禁并发执行（E1）。
//
//	env:
//	  source tools/devenv.sh micro_analytics   # MICRO_DSN_ANALYTICS + MICRO_DEV_* 注入
//	  go run ./tools/p4smoke
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	_ "github.com/go-sql-driver/mysql"

	anpb "micro-server/services/analytics/pb"
)

var (
	// platformTenant 平台租户（通知模板/站内信收件人所在租户）
	platformTenant = int64(9000000000000000001)
	// tenantID 本次运行租户（main 中赋值 = runTenant）
	tenantID int64
	// seedUID 平台管理员（seed 固定 uid）
	seedUID = int64(9000000000000000002)
)

// runTenant 每次运行独立租户：日聚合行 (tenant, date) 互不串扰，断言可精确 ==；
// 遗留运行（漏发未补）由 p4backfill 按日全租户重放自愈（台账业务键幂等）。
var runTenant int64

// idBase 每次运行唯一 ID 基（完整毫秒，跨次运行不撞 PK）。
var idBase int64

func genID(prefix int64, seq int64) int64 { return prefix + idBase*10 + seq }

var (
	kafkaBrokers = envOr("KAFKA_BROKERS", "127.0.0.1:29092")
	analyticsRPC = envOr("ANALYTICS_RPC", "127.0.0.1:8095")
	dsn          = envOr("MICRO_DSN_ANALYTICS", "")
)

var dsnNotification = replaceDB(dsn, "micro_notification")

func main() {
	logx.MustSetup(logx.LogConf{ServiceName: "p4smoke", Mode: "console"})
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), seedUID)

	if dsn == "" {
		fail("MICRO_DSN_ANALYTICS 未设置（source tools/devenv.sh micro_analytics）")
	}

	// ---- 直连 analytics RPC ----
	conn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{analyticsRPC},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
	an := anpb.NewAnalyticsClient(conn)

	db, err := sql.Open("mysql", dsn)
	must(err, "analytics db open")
	ndb, err := sql.Open("mysql", dsnNotification)
	must(err, "notification db open")
	defer func() { _ = db.Close(); _ = ndb.Close() }()

	idBase = time.Now().UnixMilli()
	runTenant = int64(910000000000000000) + idBase
	tenantID = runTenant
	ctx = ctxkit.WithUID(ctxkit.WithTenant(context.Background(), runTenant), seedUID)

	sn := fmt.Sprintf("P4%d", time.Now().UnixMilli()%100000000)
	today := time.Now().Format("2006-01-02")
	fmt.Printf("== p4smoke 分析域冒烟 sn=%s biz_date=%s ==\n", sn, today)

	// ---- 0. 种子：RECON_DIFF 通知模板 + WECHAT 渠道（幂等）----
	seedNotification(ndb)

	// ---- 1. 双侧种子：业务库直插（事实源）+ 事件（除漏发用例）----
	ids := seedBusiness(db, sn)
	publishEvents(ids, sn)
	pass("业务种子 + 事件发布（order.created 漏发 1 条：order_no=%s）", ids.orderNo3)

	// ---- 2. 读模型消费到位 ----
	waitFor(60*time.Second, time.Second, "读模型消费（orders_paid=2）", func() (bool, error) {
		out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "sales", DateStart: today, DateEnd: today})
		if err != nil {
			return false, err
		}
		for _, r := range out.Rows {
			if r.Metrics["orders_paid"] == "2" {
				return true, nil
			}
		}
		return false, nil
	})
	pass("读模型消费到位（台账幂等 + 聚合）")

	// ---- 3. 六大报表口径断言（轮询：各 topic 消费进度独立，以全队列排空为准）----
	assertSales(an, ctx, today, ids)
	assertFinance(an, ctx, today, ids)
	assertInventory(an, ctx, today, ids)
	assertWarranty(an, ctx, today, ids)
	assertSLA(an, ctx, today, ids)
	assertStation(an, ctx, today, ids)

	// ---- 4. 指标字典 ----
	dict, err := an.ListMetricDict(ctx, &anpb.ListMetricDictReq{})
	must(err, "ListMetricDict")
	if dict.Total < 18 {
		fail("指标字典不足 18（got %d）", dict.Total)
	}
	pass("指标字典 %d 条（≥18，口径可查）", dict.Total)

	// watermark 观测
	wm, err := an.ListWatermark(ctx, &anpb.ListWatermarkReq{})
	must(err, "ListWatermark")
	pass("watermark topics=%d version=%s", len(wm.List), wm.RebuildVersion)

	// ---- 5. 对账抓"上游漏发事件"（orders_created：业务 3 vs 读模型 2）----
	recon, err := an.RunRecon(ctx, &anpb.RunReconReq{ReconDate: today})
	must(err, "RunRecon")
	if recon.DiffTotal < 1 {
		fail("对账未抓到注入的漏发事件（diff=%d）——验收用例失败", recon.DiffTotal)
	}
	var diffDetail string
	err = db.QueryRow("SELECT CONCAT(metric_code,':',source_count,'/',model_count) FROM recon_diff WHERE status='OPEN' AND metric_code='orders_created' AND biz_date = ?", today).Scan(&diffDetail)
	must(err, "recon_diff 查询")
	pass("对账抓到漏发事件 diff_total=%d（%s）", recon.DiffTotal, diffDetail)

	// 差异站内信触达（notification_request → notification INBOX）
	waitFor(30*time.Second, time.Second, "差异站内信落库", func() (bool, error) {
		var n int
		err := ndb.QueryRow("SELECT COUNT(*) FROM message WHERE biz_type = 'tpl:RECON_DIFF' AND tenant_id = ?", platformTenant).Scan(&n)
		if err != nil {
			return false, nil
		}
		return n >= 1, nil
	})
	pass("差异站内信触达（micro_notification.message）")

	// ---- 6. p4backfill 补投递（E10 禁裸删：业务库重放）----
	cmd := exec.Command("go", "run", "./tools/p4backfill", "-date", today)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fail("p4backfill 执行失败: %v\n%s", err, string(out))
	}
	pass("p4backfill 补投递完成\n%s", indent(string(out)))

	// ---- 7. 复跑对账：差异率 0（闭环）----
	waitFor(60*time.Second, time.Second, "补投递消费（orders_created=3）", func() (bool, error) {
		r, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "sales", DateStart: today, DateEnd: today})
		if err != nil {
			return false, err
		}
		for _, row := range r.Rows {
			if row.Metrics["orders_created"] == "3" {
				return true, nil
			}
		}
		return false, nil
	})
	recon2, err := an.RunRecon(ctx, &anpb.RunReconReq{ReconDate: today})
	must(err, "RunRecon 复跑")
	if recon2.DiffTotal != 0 {
		fail("复跑对账差异率非 0（diff=%d）——业务键幂等或补投递失败", recon2.DiffTotal)
	}
	pass("复跑对账差异率 0（p4quality 口径；台账业务键幂等防重放）")

	// ---- 8. 异步导出 CSV/XLSX/PDF → 下载中心 ----
	exportWait(an, ctx, today, today, "sales", "CSV")
	exportWait(an, ctx, today, today, "sla", "XLSX")
	exportWait(an, ctx, today, today, "warranty", "PDF")

	fmt.Printf("== p4smoke 全链通过 ==\n")
}

// ---- 断言 ----

func getSales(an anpb.AnalyticsClient, ctx context.Context, today string) map[string]string {
	out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "sales", DateStart: today, DateEnd: today})
	must(err, "GetReport sales")
	if len(out.Rows) != 1 {
		fail("sales 行数异常: %d", len(out.Rows))
	}
	return out.Rows[0].Metrics
}

func assertSales(an anpb.AnalyticsClient, ctx context.Context, today string, ids seedIDs) {
	m := getSales(an, ctx, today)
	expect(m, "orders_created", "2") // O3 漏发用例：暂为 2
	expect(m, "orders_paid", "2")
	expect(m, "orders_cancelled", "1")
	expect(m, "gmv_paid", ids.gmv)
	expect(m, "shipments_signed", "1")
	pass("销售日报口径（orders_paid=2 gmv=%s 签收=1）", ids.gmv)
}

func assertFinance(an anpb.AnalyticsClient, ctx context.Context, today string, ids seedIDs) {
	out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "finance", DateStart: today, DateEnd: today})
	must(err, "GetReport finance")
	found := false
	for _, r := range out.Rows {
		if r.Channel == "WECHAT" {
			found = true
			if r.Metrics["payments_count"] != "2" || r.Metrics["paid_amount"] != ids.gmv || r.Metrics["refunds_count"] != "1" {
				fail("财务日报口径异常: %v", r.Metrics)
			}
		}
	}
	if !found {
		fail("财务日报无 WECHAT 渠道行")
	}
	pass("财务日报口径（渠道维度 WECHAT 支付 2 退款 1）")
}

func assertInventory(an anpb.AnalyticsClient, ctx context.Context, today string, ids seedIDs) {
	out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "inventory", DateStart: today, DateEnd: today, WarehouseId: ids.warehouseID})
	must(err, "GetReport inventory")
	if len(out.Rows) != 1 {
		fail("inventory 行数异常: %d", len(out.Rows))
	}
	m := out.Rows[0].Metrics
	expect(m, "stock_in_qty", "60")
	expect(m, "stock_out_qty", "5")
	pass("库存日报口径（仓库维度 入 60 出 5）")
}

func assertWarranty(an anpb.AnalyticsClient, ctx context.Context, today string, _ seedIDs) {
	out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "warranty", DateStart: today, DateEnd: today})
	must(err, "GetReport warranty")
	if len(out.Rows) != 1 {
		fail("warranty 行数异常: %d", len(out.Rows))
	}
	m := out.Rows[0].Metrics
	expect(m, "warranties_started", "1")
	expect(m, "devices_activated", "2")
	pass("质保日报口径（起算 1 激活 2）")
}

func assertSLA(an anpb.AnalyticsClient, ctx context.Context, today string, ids seedIDs) {
	waitFor(30*time.Second, 500*time.Millisecond, "SLA 日报聚合（alerts_total=2）", func() (bool, error) {
		out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "sla", DateStart: today, DateEnd: today, StationId: ids.stationID})
		if err != nil {
			return false, err
		}
		for _, r := range out.Rows {
			if r.Metrics["alerts_total"] == "2" && r.Metrics["alerts_p1"] == "1" {
				return true, nil
			}
		}
		return false, nil
	})
	pass("SLA 日报口径（场站维度 告警 2 P1=1）")
}

func assertStation(an anpb.AnalyticsClient, ctx context.Context, today string, ids seedIDs) {
	waitFor(30*time.Second, 500*time.Millisecond, "场站日报聚合（station_alerts=2）", func() (bool, error) {
		out, err := an.GetReport(ctx, &anpb.GetReportReq{ReportType: "station", DateStart: today, DateEnd: today, StationId: ids.stationID})
		if err != nil {
			return false, err
		}
		for _, r := range out.Rows {
			if r.Metrics["station_alerts"] == "2" && r.Metrics["stations_created"] == "1" {
				return true, nil
			}
		}
		return false, nil
	})
	pass("场站日报口径（新建 1 告警 2）")
}

func expect(m map[string]string, key, want string) {
	if m[key] != want {
		fail("报表口径 %s: want %s got %s", key, want, m[key])
	}
}

func exportWait(an anpb.AnalyticsClient, ctx context.Context, start, end, reportType, format string) {
	req, err := an.RequestExport(ctx, &anpb.RequestExportReq{
		ReportType: reportType, Format: format, DateStart: start, DateEnd: end,
	})
	must(err, "RequestExport "+format)
	id := req.ExportId
	waitFor(60*time.Second, 500*time.Millisecond,
		fmt.Sprintf("导出 DONE %s/%s task=%s", reportType, format, id), func() (bool, error) {
			t, err := an.GetExportTask(ctx, &anpb.GetExportTaskReq{ExportId: id})
			if err != nil {
				return false, err
			}
			if t.Task.Status == "FAILED" {
				return false, fmt.Errorf("导出失败: %s", t.Task.Error)
			}
			return t.Task.Status == "DONE", nil
		})
	t, _ := an.GetExportTask(ctx, &anpb.GetExportTaskReq{ExportId: id})
	if t.Task.FileId == "0" || t.Task.FileId == "" {
		fail("导出产物未归档 task=%s", id)
	}
	pass("异步导出 %s/%s 完成 rows=%d cost=%dms file=%s（下载中心）",
		reportType, format, t.Task.RowCount, t.Task.CostMs, t.Task.FileId)
}

// ---- 种子 ----

type seedIDs struct {
	orderNo1, orderNo2, orderNo3 string
	paymentNo1, paymentNo2       string
	refundNo                     string
	warrantyNo                   string
	sn1, sn2                     string
	stationNo                    string
	shipmentNo                   string
	stationID                    int64
	warehouseID                  int64
	alertIDs                     []int64
	gmv                          string
}

// seedBusiness 业务库直插（事实源；读模型从不直查业务库——只有对账/backfill 直查）。
func seedBusiness(db *sql.DB, sn string) seedIDs {
	ids := seedIDs{
		orderNo1: sn + "O1", orderNo2: sn + "O2", orderNo3: sn + "O3",
		paymentNo1: sn + "P1", paymentNo2: sn + "P2",
		refundNo: sn + "R1", warrantyNo: sn + "W1",
		sn1: sn + "D1", sn2: sn + "D2",
		stationNo:   sn + "S1",
		shipmentNo:  sn + "H1",
		stationID:   int64(912000000000000000) + idBase,
		warehouseID: int64(911000000000000000) + idBase,
		gmv:         "300.50",
	}
	ids.alertIDs = []int64{genID(913000000000000000, 0), genID(913000000000000000, 1)}

	// 订单 ×3（O3 为漏发用例：status CANCELLED）
	for i, no := range []string{ids.orderNo1, ids.orderNo2, ids.orderNo3} {
		status, cancelReason := "CREATED", ""
		if i == 2 {
			status, cancelReason = "CANCELLED", "p4smoke 用户取消"
		}
		amount := []string{"100.50", "200.00", "50.00"}[i]
		oid := genID(914000000000000000, int64(i))
		_, err := db.Exec(`INSERT INTO micro_order.`+"`order`"+`
			(order_id, order_no, type, status, total_amount, buyer_party_id, cancel_reason, tenant_id, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,NOW(),NOW())`,
			oid, no, "DEVICE", status, amount, 1, cancelReason, tenantID)
		must(err, "seed order "+no)
	}

	// 支付 ×2（PAID，今天 paid_at）
	for i, no := range []string{ids.paymentNo1, ids.paymentNo2} {
		orderNo := []string{ids.orderNo1, ids.orderNo2}[i]
		amount := []string{"100.50", "200.00"}[i]
		pid := genID(915000000000000000, int64(i))
		_, err := db.Exec(`INSERT INTO micro_finance.payment
			(payment_id, payment_no, order_no, channel, amount, paid_amount, status, paid_at, tenant_id, created_at, updated_at)
			VALUES (?,?,?,?,?,?, 'PAID', NOW(3), ?, NOW(), NOW())`,
			pid, no, orderNo, "WECHAT", amount, amount, tenantID)
		must(err, "seed payment "+no)
	}

	// 退款 ×1（SUCCESS）
	_, err := db.Exec(`INSERT INTO micro_finance.refund
		(refund_id, refund_no, payment_no, order_no, amount, channel, status, tenant_id, created_at, updated_at)
		VALUES (?,?,?,?,?, 'WECHAT', 'SUCCESS', ?, NOW(), NOW())`,
		genID(916000000000000000, 0), ids.refundNo, ids.paymentNo1, ids.orderNo1, "50.00", tenantID)
	must(err, "seed refund")

	// 库存流水：STOCK_IN ×3（10/20/30）+ DEDUCT ×1（5）
	recs := []struct {
		bizType string
		qty     int
		bizNo   string
	}{
		{"STOCK_IN", 10, sn + "I1"}, {"STOCK_IN", 20, sn + "I2"}, {"STOCK_IN", 30, sn + "I3"}, {"DEDUCT", 5, sn + "I4"},
	}
	for i, r := range recs {
		_, err := db.Exec(`INSERT INTO micro_inventory.stock_record
			(record_id, inventory_id, warehouse_id, sku_id, biz_type, biz_no, qty, before_available, after_available, tenant_id, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,NOW(),NOW())`,
			genID(917000000000000000, int64(i)),
			genID(917100000000000000, 0), ids.warehouseID, genID(917100000000000000, 1), r.bizType, r.bizNo, r.qty, 0, r.qty, tenantID)
		must(err, "seed stock_record "+r.bizNo)
	}

	// 质保 ×1（DEVICE 层，今天起算）
	endAt := time.Now().AddDate(0, 12, 0).Format("2006-01-02 15:04:05")
	_, err = db.Exec(`INSERT INTO micro_contract.warranty
		(warranty_id, warranty_no, level, target_type, target_id, target_key, status, start_at, end_at, months, source_type, tenant_id, created_at, updated_at)
		VALUES (?,?, 'DEVICE', 'DEVICE', 0, ?, 'ACTIVE', NOW(3), ?, 12, 'ORDER', ?, NOW(), NOW())`,
		genID(918000000000000000, 0), ids.warrantyNo, ids.sn1, endAt, tenantID)
	must(err, "seed warranty")

	// 设备激活日志 ×2
	for i, snD := range []string{ids.sn1, ids.sn2} {
		_, err := db.Exec(`INSERT INTO micro_device.device_lifecycle_log
			(log_id, device_id, sn, from_status, to_status, event_type, event_id, tenant_id, created_at)
			VALUES (?, ?, ?, '', 'ACTIVATED', 'device.activated', ?, ?, NOW())`,
			genID(919000000000000000, int64(i)),
			genID(919100000000000000, int64(i)), snD, snD, tenantID)
		must(err, "seed lifecycle "+snD)
	}

	// 场站 ×1（今天创建）
	_, err = db.Exec(`INSERT INTO micro_station.station
		(station_id, station_no, name, type, status, tenant_id, created_at, updated_at)
		VALUES (?,?, 'p4smoke场站', 'ESS', 'ACTIVE', ?, NOW(), NOW())`,
		ids.stationID, ids.stationNo, tenantID)
	must(err, "seed station")

	// 发运 ×1（SIGNED 今天）
	_, err = db.Exec(`INSERT INTO micro_order.shipment
		(shipment_id, shipment_no, order_id, order_no, warehouse_id, status, signed_at, tenant_id, created_at, updated_at)
		VALUES (?,?,?,?,?, 'SIGNED', NOW(3), ?, NOW(), NOW())`,
		genID(919200000000000000, 0), ids.shipmentNo,
		genID(914000000000000000, 0), ids.orderNo1, ids.warehouseID, tenantID)
	must(err, "seed shipment")

	// 告警 ×2（P1/P3，场站 903 归属）
	levels := []string{"P1", "P3"}
	for i, aid := range ids.alertIDs {
		_, err := db.Exec(`INSERT INTO micro_ops.alert
			(alert_id, level, source, metric, station_id, station_name, sn, title, status, aggregate_count, first_seen_at, last_seen_at, tenant_id, created_at, updated_at)
			VALUES (?,?, 'TELEMETRY', 'cell_temp', ?, 'p4smoke场站', ?, ?, 'OPEN', 1, NOW(3), NOW(3), ?, NOW(), NOW())`,
			aid, levels[i], ids.stationID, ids.sn1, "p4smoke告警"+sn+"-"+levels[i], tenantID)
		must(err, "seed alert "+levels[i])
	}

	pass("业务库种子就绪（order×3 payment×2 refund×1 stock×4 warranty×1 lifecycle×2 station×1 shipment×1 alert×2）")
	return ids
}

// publishEvents 发布 17 条信封事件（漏发 order.created(O3) 不发——对账验收用例）。
func publishEvents(ids seedIDs, sn string) {
	now := time.Now().Format(time.RFC3339Nano)
	pub("order_created", ids.orderNo1, "order.created", ids.orderNo1, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo1, "order_type": "DEVICE",
		"total_amount": "100.50", "buyer_party_id": 1,
	})
	pub("order_created", ids.orderNo2, "order.created", ids.orderNo2, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo2, "order_type": "DEVICE",
		"total_amount": "200.00", "buyer_party_id": 1,
	})
	// O3 漏发（order.created 不发）——cancelled/paid 正常发
	pub("order_paid", ids.orderNo1, "order.paid", ids.orderNo1, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo1, "payment_no": ids.paymentNo1,
		"channel": "WECHAT", "paid_amount": "100.50", "paid_at": time.Now().UnixMilli(),
	})
	pub("order_paid", ids.orderNo2, "order.paid", ids.orderNo2, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo2, "payment_no": ids.paymentNo2,
		"channel": "WECHAT", "paid_amount": "200.00", "paid_at": time.Now().UnixMilli(),
	})
	pub("order_cancelled", ids.orderNo3, "order.cancelled", ids.orderNo3, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo3, "reason": "p4smoke", "cancelled_by": "USER",
	})
	for i, r := range []struct {
		qty int
		no  string
		out bool
	}{
		{10, sn + "I1", false}, {20, sn + "I2", false}, {30, sn + "I3", false}, {5, sn + "I4", true},
	} {
		tp := "stock.in"
		topic := "stock_in"
		if r.out {
			tp = "stock.out"
			topic = "stock_out"
		}
		pub(topic, r.no, tp, r.no, map[string]any{
			"tenant_id": tenantID, "warehouse_id": ids.warehouseID, "sku_id": 930000000000000902,
			"qty": r.qty, "biz_no": r.no, "sns": []string{},
		})
		_ = i
	}
	pub("payment_refunded", ids.refundNo, "payment.refunded", ids.refundNo, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo1, "payment_no": ids.paymentNo1,
		"return_no": "", "refund_no": ids.refundNo, "amount": "50.00", "channel": "WECHAT",
	})
	pub("warranty_started", ids.warrantyNo, "warranty.started", ids.warrantyNo, map[string]any{
		"tenant_id": tenantID, "warranty_no": ids.warrantyNo, "level": "DEVICE",
		"target_key": ids.sn1, "start_at": time.Now().UnixMilli(),
		"end_at": time.Now().AddDate(0, 12, 0).UnixMilli(), "months": 12,
	})
	for _, snD := range []string{ids.sn1, ids.sn2} {
		pub("device_activated", snD, "device.activated", snD, map[string]any{
			"tenant_id": tenantID, "sn": snD, "activated_at": time.Now().UnixMilli(),
		})
	}
	pub("shipment_signed", ids.shipmentNo, "shipment.signed", ids.shipmentNo, map[string]any{
		"tenant_id": tenantID, "order_no": ids.orderNo1, "shipment_no": ids.shipmentNo,
		"sns": []string{}, "signed_by": "p4smoke", "signed_at": time.Now().UnixMilli(),
	})
	pub("station_created", ids.stationNo, "station.created", ids.stationNo, map[string]any{
		"tenant_id": tenantID, "station_id": ids.stationID, "station_no": ids.stationNo,
		"name": "p4smoke场站", "order_no": "",
	})
	pub("alert_created", strconv.FormatInt(ids.alertIDs[0], 10), "alert.created",
		strconv.FormatInt(ids.alertIDs[0], 10), map[string]any{
			"alert_id": ids.alertIDs[0], "tenant_id": tenantID, "station_id": ids.stationID,
			"level": "P1", "source": "TELEMETRY", "metric": "cell_temp",
			"title": "p4smoke告警" + sn + "-P1", "first_seen_at": time.Now().UnixMilli(),
		})
	pub("alert_created", strconv.FormatInt(ids.alertIDs[1], 10), "alert.created",
		strconv.FormatInt(ids.alertIDs[1], 10), map[string]any{
			"alert_id": ids.alertIDs[1], "tenant_id": tenantID, "station_id": ids.stationID,
			"level": "P3", "source": "TELEMETRY", "metric": "cell_temp",
			"title": "p4smoke告警" + sn + "-P3", "first_seen_at": time.Now().UnixMilli(),
		})
	_ = now
	pass("信封事件发布完成（17-1 漏发）")
}

// pub 发信封事件（与 eventbus.Envelope 同形；payload 为业务字段裸 JSON——analytics 按类型解析）。
func pub(topic, key, eventType, bizKey string, payload map[string]any) {
	env := map[string]any{
		"event_id":    newEventID(),
		"event_type":  eventType,
		"tenant_id":   tenantID,
		"trace_id":    "",
		"occurred_at": time.Now().Format(time.RFC3339Nano),
		"payload":     payload,
	}
	raw, _ := json.Marshal(env)
	w := &kafka.Writer{Addr: kafka.TCP(kafkaBrokers), Topic: topic, AllowAutoTopicCreation: false, Balancer: &kafka.Hash{}}
	if err := w.WriteMessages(context.Background(), kafka.Message{Key: []byte(bizKey), Value: raw}); err != nil {
		fail("Kafka 生产 %s 失败: %v", topic, err)
	}
	_ = w.Close()
}

// seedNotification RECON_DIFF 模板 + WECHAT 渠道（幂等种子，micro_notification）。
func seedNotification(db *sql.DB) {
	_, err := db.Exec(`insert ignore into notify_channel (channel_id, code, provider, config, status, tenant_id)
		values (1, 'WECHAT', 'wechat', '{}', 1, ?)`, platformTenant)
	must(err, "seed notify_channel")
	_, err = db.Exec(`insert ignore into notify_template (template_id, code, title_template, content_template, channel, status, tenant_id)
		values (200, 'RECON_DIFF', '读模型对账差异 {{biz_date}}', '对账发现 {{diff_total}} 处差异，请核查并走补投递重放（禁裸删）', 'WECHAT', 1, ?)`, platformTenant)
	must(err, "seed notify_template RECON_DIFF")
	pass("RECON_DIFF 通知模板种子就绪")
}

// ---- 基础设施（与 p3smoke 同款） ----

func replaceDB(dsn, newDB string) string {
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == '/' {
			rest := dsn[i+1:]
			for j := 0; j < len(rest); j++ {
				if rest[j] == '?' {
					return dsn[:i+1] + newDB + rest[j:]
				}
			}
			return dsn[:i+1] + newDB
		}
	}
	return dsn
}

func waitFor(total time.Duration, interval time.Duration, name string, probe func() (bool, error)) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		ok, err := probe()
		if ok {
			return
		}
		_ = err
		time.Sleep(interval)
	}
	fail("等待超时: %s", name)
}

func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func indent(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "p4smoke FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(format string, args ...any) {
	fmt.Printf("  ✅ "+format+"\n", args...)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "p4smoke FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
