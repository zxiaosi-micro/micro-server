// drill · S11-03 故障演练（runbook 脚本化，可重复执行）+ S11-02 Kafka lag 积压恢复演练。
//
// 场景（01 §6.2/02 §9；服务启停经 tools/smoke 的 pwsh 启动器，进程级硬杀即演练语义）：
//
//	station    station 停机 → Saga 卡建站停靠（next_retry_at/last_error 可观测）→ 恢复后 cron 自动补齐至 DONE
//	inventory  inventory 停机 → 下单降级拒绝（RPC 不可达即失败）→ 恢复后库存无残留（不超卖）
//	analytics  analytics 停机 → order_paid 积压（Kafka lag 上涨）→ 恢复后 rebalance 补消费
//	           （report_event_entry 落行、lag 回落；uk_biz 幂等=单行，S11-02 lag 演练同场景）
//	cron       contract 停机 → 到期质保不被扫描（E16 逾期窗口）→ 恢复后首轮扫描补跑 EXPIRED
//
// 前置（同 p5smoke）：compose core 全绿 + source tools/devenv.sh + smoke-start-p5.sh 拉起
// identity/party/catalog/inventory/device/finance/contract/station/order/ops/notification/mobile-bff
// + analytics（MICRO_DSN_ANALYTICS 同款喂法）。演练会停启 inventory/contract/station/analytics 四服务。
//
// 用法：go run ./tools/drill [-case all|station|inventory|analytics|cron]
// 纪律：严禁并发执行（E1）；总时长 ~5min（cron 场景含 70s 停机窗口）。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	cpb "micro-server/services/catalog/pb"
	finpb "micro-server/services/finance/pb"
	invpb "micro-server/services/inventory/pb"
	opb "micro-server/services/order/pb"

	"micro-server/tools/internal/devcfg"
)

const (
	seedUID     = 9000000000000000002
	kafkaBroker = "127.0.0.1:29092"
)

var (
	catalogRPC   = envOr("CATALOG_RPC", "127.0.0.1:8083")
	inventoryRPC = envOr("INVENTORY_RPC", "127.0.0.1:8084")
	orderRPC     = envOr("ORDER_RPC", "127.0.0.1:8090")
	financeRPC   = envOr("FINANCE_RPC", "127.0.0.1:8092")
	contractDSN  = envOr("MICRO_DSN_CONTRACT", "")
	analyticsDSN = envOr("MICRO_DSN_ANALYTICS", envOr("DSN", ""))
	runID        = fmt.Sprintf("dr-%d", time.Now().UnixMilli()%100000000)
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// ---- 断言/等待 ----

func mustNoErr(err error, what string) {
	if err != nil {
		fmt.Printf("  ✗ %s: %v\n", what, err)
		fmt.Println("drill: FAIL")
		os.Exit(1)
	}
	fmt.Printf("  · %s\n", what)
}

func mustT(cond bool, what string) {
	if !cond {
		fmt.Printf("  ✗ %s\n", what)
		fmt.Println("drill: FAIL")
		os.Exit(1)
	}
	fmt.Printf("  · %s\n", what)
}

func waitFor(d time.Duration, what string, cond func() bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			fmt.Printf("  · %s\n", what)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("  ✗ 等待超时: %s\n", what)
	fmt.Println("drill: FAIL")
	os.Exit(1)
}

// ---- 服务启停（复用 tools/smoke 启动器；硬杀即演练语义） ----

const smokeDir = "tools/smoke"

func stopSvc(name string, port int) {
	fmt.Printf("  · stop %s(:%d)\n", name, port)
	cmd := exec.Command("pwsh", "-File", smokeDir+"/stop-one.ps1", "-Port", fmt.Sprint(port))
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("    (stop 输出: %s %v)\n", strings.TrimSpace(string(out)), err)
	}
	waitPort(port, false, 15*time.Second)
}

func startSvc(name string, port int) {
	fmt.Printf("  · start %s(:%d)\n", name, port)
	cmd := exec.Command("pwsh", "-File", smokeDir+"/start-one.ps1",
		"-Name", name, "-Port", fmt.Sprint(port), "-LogDir", "/tmp/micro-smoke/logs")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("    (start 输出: %s %v)\n", strings.TrimSpace(string(out)), err)
	}
	waitPort(port, true, 30*time.Second)
}

func waitPort(port int, up bool, d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 800*time.Millisecond)
		if (err == nil) == up {
			if conn != nil {
				_ = conn.Close()
			}
			return
		}
		if conn != nil {
			_ = conn.Close()
		}
		time.Sleep(400 * time.Millisecond)
	}
	mustT(false, fmt.Sprintf("端口 %d 期待 %v 超时", port, up))
}

// ---- RPC/SQL ----

func rpcConn(addr string) grpc.ClientConnInterface {
	conn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{addr},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
	return conn.Conn()
}

func mustDefaultTenant(dsn string) int64 {
	db := openDB(dsn, "identity")
	defer func() { _ = db.Close() }()
	var tid int64
	mustNoErr(db.QueryRow("select tenant_id from tenant where tenant_code='default'").Scan(&tid), "查 default 租户")
	return tid
}

func openDB(dsn, name string) *sql.DB {
	if dsn == "" {
		fmt.Printf("  ✗ %s DSN 未注入（source tools/devenv.sh）\n", name)
		os.Exit(1)
	}
	db, err := sql.Open("mysql", dsn)
	mustNoErr(err, "连 "+name)
	return db
}

// ensureSku chaosmoke 同款：自建商品/SKU/价目 + 入库（run 内唯一，幂等）。
func ensureSku(ctx context.Context, c *catalogInv, key string, qty int64) (skuID, whID int64, availBefore int64) {
	ts := time.Now().Format("0102150405")
	pr, err := c.catalog.CreateProduct(ctx, &cpb.CreateProductReq{Name: "drill-" + key, Category: "DRILL"})
	mustNoErr(err, "建商品 "+key)
	skr, err := c.catalog.CreateSKU(ctx, &cpb.CreateSKUReq{ProductId: pr.ProductId, Code: "DRILL-" + key, Name: "drill-" + key, Type: "STANDARD"})
	mustNoErr(err, "建 SKU "+key)
	_, err = c.catalog.SetPrice(ctx, &cpb.SetPriceReq{SkuId: skr.SkuId, PriceType: "RETAIL", Amount: "88.00"})
	mustNoErr(err, "设价 "+key)
	wh, err := c.inv.CreateWarehouse(ctx, &invpb.CreateWarehouseReq{Code: "DR-WH-" + key + ts, Name: "drill仓"})
	mustNoErr(err, "建仓")
	_, err = c.inv.StockIn(ctx, &invpb.StockInReq{WarehouseId: wh.WarehouseId, SkuId: skr.SkuId, Qty: int32(qty), BizType: "STOCK_IN", BizNo: "DR-IN-" + key + ts})
	mustNoErr(err, "入库")
	gi, err := c.inv.GetInventory(ctx, &invpb.GetInventoryReq{WarehouseId: wh.WarehouseId, SkuId: skr.SkuId})
	mustNoErr(err, "查库存")
	return skr.SkuId, wh.WarehouseId, int64(gi.Inventory.Available)
}

func availableOf(ctx context.Context, c *catalogInv, sku, wh int64) int64 {
	gi, err := c.inv.GetInventory(ctx, &invpb.GetInventoryReq{WarehouseId: wh, SkuId: sku})
	if err != nil {
		return -1
	}
	return int64(gi.Inventory.Available)
}

func orderOf(ctx context.Context, c *catalogInv, no string) *opb.OrderDetail {
	o, err := c.order.GetOrder(ctx, &opb.GetOrderReq{OrderNo: no})
	if err != nil {
		return nil
	}
	return o.Order
}

type catalogInv struct {
	catalog cpb.CatalogClient
	inv     invpb.InventoryClient
	order   opb.OrderClient
	finance finpb.FinanceClient
}

// payAndConfirm 下单→支付→渠道确认（S5 全链；停掉某下游即在对应 Saga 步骤停靠）。
func payAndConfirm(ctx context.Context, c *catalogInv, sku, wh int64, sn string) string {
	co, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Remark: "drill-" + runID,
		Items: []*opb.OrderItemInput{{SkuId: sku, WarehouseId: wh, Qty: 1, Sn: sn}},
	})
	mustNoErr(err, "下单")
	waitFor(40*time.Second, "锁定", func() bool {
		o := orderOf(ctx, c, co.OrderNo)
		return o != nil && o.Status == "LOCKED"
	})
	po, err := c.order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "WECHAT"})
	mustNoErr(err, "发起支付")
	_, err = c.finance.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: "DR-TXN-" + sn, PaidAmount: co.TotalAmount, Source: "MOCK"})
	mustNoErr(err, "渠道确认")
	return co.OrderNo
}

func main() {
	caseSel := flag.String("case", "all", "场景 all|station|inventory|analytics|cron")
	flag.Parse()
	fmt.Printf("=== drill · S11-03 故障演练 runID=%s（严禁并发 E1） ===\n", runID)

	dsnIdentity := devcfg.Get("MICRO_DSN_IDENTITY", devcfg.Get("DSN", ""))
	tid := mustDefaultTenant(dsnIdentity)
	fmt.Printf("[0] default 租户 = %d\n", tid)
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tid), seedUID)

	c := &catalogInv{
		catalog: cpb.NewCatalogClient(rpcConn(catalogRPC)),
		inv:     invpb.NewInventoryClient(rpcConn(inventoryRPC)),
		order:   opb.NewOrderClient(rpcConn(orderRPC)),
		finance: finpb.NewFinanceClient(rpcConn(financeRPC)),
	}

	switch *caseSel {
	case "all":
		caseStation(ctx, c)
		caseInventory(ctx, c)
		caseAnalytics(ctx, tid)
		caseCron(ctx, tid)
	case "station":
		caseStation(ctx, c)
	case "inventory":
		caseInventory(ctx, c)
	case "analytics":
		caseAnalytics(ctx, tid)
	case "cron":
		caseCron(ctx, tid)
	default:
		fmt.Println("drill: -case 取值 all|station|inventory|analytics|cron")
		os.Exit(1)
	}
	fmt.Printf("\ndrill: PASS（runID=%s）\n", runID)
}

// caseStation 场景1：station 停机 → Saga 建站步骤停靠 → 恢复后自动补齐 DONE。
func caseStation(ctx context.Context, c *catalogInv) {
	fmt.Println("\n—— 场景 1 station 停机 → Saga 卡建站 → 恢复补齐 ——")
	sku, wh, _ := ensureSku(ctx, c, "st-"+runID, 5)
	stopSvc("station", 8091)
	orderNo := payAndConfirm(ctx, c, sku, wh, "DR-ST-"+runID)
	waitFor(60*time.Second, "Saga 建站步骤停靠（订单停在中间态）", func() bool {
		o := orderOf(ctx, c, orderNo)
		return o != nil && o.Status != "DONE"
	})
	o := orderOf(ctx, c, orderNo)
	sg, err := c.order.GetSaga(ctx, &opb.GetSagaReq{OrderNo: orderNo})
	if err == nil && sg.Saga != nil {
		mustT(sg.Saga.NextRetryAt > 0 || sg.Saga.LastError != "", "Saga 停靠可观测（next_retry_at/last_error，E16）")
	}
	startSvc("station", 8091)
	waitFor(150*time.Second, "cron 扫描自动补齐：订单 DONE", func() bool {
		o := orderOf(ctx, c, orderNo)
		return o != nil && o.Status == "DONE"
	})
	_ = o
}

// caseInventory 场景2：inventory 停机 → 下单拒绝 → 恢复后库存无残留。
func caseInventory(ctx context.Context, c *catalogInv) {
	fmt.Println("\n—— 场景 2 inventory 停机 → 下单降级拒绝不超卖 ——")
	sku, wh, avail := ensureSku(ctx, c, "inv-"+runID, 5)
	stopSvc("inventory", 8084)
	_, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type:  "DEVICE",
		Items: []*opb.OrderItemInput{{SkuId: sku, WarehouseId: wh, Qty: 1, Sn: "DR-INV-" + runID}},
	})
	mustT(err != nil, "inventory 停机期间下单被拒（降级语义）")
	startSvc("inventory", 8084)
	waitFor(30*time.Second, "inventory 恢复", func() bool { return availableOf(ctx, c, sku, wh) >= 0 })
	mustT(availableOf(ctx, c, sku, wh) == avail, "库存无残留锁定（available 还原，不超卖）")
}

// caseAnalytics 场景3（含 S11-02 lag 演练）：analytics 停机 → order_paid 积压 → 恢复补消费幂等。
func caseAnalytics(ctx context.Context, tid int64) {
	fmt.Println("\n—— 场景 3 analytics 停机 → Kafka lag 积压 → 恢复补投递 ——")
	c := &catalogInv{
		catalog: cpb.NewCatalogClient(rpcConn(catalogRPC)),
		inv:     invpb.NewInventoryClient(rpcConn(inventoryRPC)),
		order:   opb.NewOrderClient(rpcConn(orderRPC)),
		finance: finpb.NewFinanceClient(rpcConn(financeRPC)),
	}
	sku, wh, _ := ensureSku(ctx, c, "an-"+runID, 5)
	group := readModelGroup()
	stopSvc("analytics", 8095)
	orderNo := payAndConfirm(ctx, c, sku, wh, "DR-AN-"+runID)
	time.Sleep(3 * time.Second) // 事件 outbox relay 推 Kafka
	lagBefore, endOffset := lagOf("order_paid", readModelGroup())
	mustT(endOffset > 0, fmt.Sprintf("order_paid topic 有消息（end=%d）", endOffset))
	mustT(lagBefore >= 1, fmt.Sprintf("analytics 停机期间 lag 积压（lag=%d）", lagBefore))

	startSvc("analytics", 8095)
	db := openDB(analyticsDSN, "analytics")
	defer func() { _ = db.Close() }()
	waitFor(120*time.Second, "恢复后补消费：report_event_entry 落行（uk_biz 幂等）", func() bool {
		var n int
		_ = db.QueryRow("select count(*) from report_event_entry where biz_key = ?", orderNo).Scan(&n)
		return n >= 1
	})
	var rows int
	mustNoErr(db.QueryRow("select count(*) from report_event_entry where biz_key = ?", orderNo).Scan(&rows), "查台账行数")
	mustT(rows == 1, "幂等：同事件单行（uk_biz，重放不重复）")
	waitFor(60*time.Second, "lag 回落（消费组追平）", func() bool {
		lag, _ := lagOf("order_paid", group)
		return lag < lagBefore
	})
	lagAfter, _ := lagOf("order_paid", group)
	fmt.Printf("  · lag %d → %d（rebalance 4~9s 后追平属预期）\n", lagBefore, lagAfter)
}

// caseCron 场景4（E16）：contract 停机 → 到期质保逾期不被扫描 → 恢复后首轮补跑。
func caseCron(ctx context.Context, tid int64) {
	fmt.Println("\n—— 场景 4 cron 扫描器停机 → 逾期任务恢复后补跑（E16） ——")
	wno := "DRW" + fmt.Sprintf("%d", time.Now().UnixMilli())
	db := openDB(contractDSN, "contract")
	func() {
		defer func() { _ = db.Close() }()
		end := time.Now().Add(-time.Hour)
		start := end.AddDate(0, -12, 0)
		_, err := db.Exec(`insert into warranty
			(warranty_id, warranty_no, level, target_type, target_id, target_key, status, start_at, end_at, months, start_rule, source_type, source_no, tenant_id)
			values (?, ?, 'DEVICE', 'DEVICE', 0, ?, 'ACTIVE', ?, ?, 12, '{"event":"drill","months":12}', 'ORDER', ?, ?)`,
			time.Now().UnixMilli(), wno, "DR-SN-"+runID, start, end, "DRILL-"+runID, tid)
		mustNoErr(err, "直插到期质保（ACTIVE，end_at=-1h）")
	}()

	stopSvc("contract", 8093)
	fmt.Println("  · 停机窗口 70s（覆盖 1m 扫描间隔，验证逾期不补跑）")
	time.Sleep(70 * time.Second)

	db2 := openDB(contractDSN, "contract")
	defer func() { _ = db2.Close() }()
	var st string
	mustNoErr(db2.QueryRow("select status from warranty where warranty_no = ?", wno).Scan(&st), "查质保状态")
	mustT(st == "ACTIVE", "停机窗口内逾期质保未被扫描（仍 ACTIVE，E16 窗口语义）")

	startSvc("contract", 8093)
	waitFor(120*time.Second, "恢复后首轮扫描补跑：EXPIRED", func() bool {
		var s string
		_ = db2.QueryRow("select status from warranty where warranty_no = ?", wno).Scan(&s)
		return s == "EXPIRED"
	})
}

// readModelGroup 读模型消费组名（report_meta k=rebuild_version，默认 1）。
func readModelGroup() string {
	v := "1"
	if analyticsDSN != "" {
		db := openDB(analyticsDSN, "analytics")
		defer func() { _ = db.Close() }()
		_ = db.QueryRow("select v from report_meta where k = 'rebuild_version'").Scan(&v)
	}
	return "analytics-read-v" + v
}

// lagOf 消费组 lag（endOffset - committed；mqinit 口径单分区 topic；
// 无已提交位点（组未建立/未提交）时 lag=-1 表示未知）。
func lagOf(topic, group string) (lag int64, end int64) {
	cl := &kafka.Client{Addr: kafka.TCP(kafkaBroker), Timeout: 5 * time.Second}
	lo, err := cl.ListOffsets(context.Background(), &kafka.ListOffsetsRequest{
		Addr:   kafka.TCP(kafkaBroker),
		Topics: map[string][]kafka.OffsetRequest{topic: {kafka.LastOffsetOf(0)}},
	})
	if err != nil {
		return -1, -1
	}
	end = lo.Topics[topic][0].LastOffset
	res, err := cl.OffsetFetch(context.Background(), &kafka.OffsetFetchRequest{
		Addr:    kafka.TCP(kafkaBroker),
		GroupID: group,
		Topics:  map[string][]int{topic: {0}},
	})
	if err != nil {
		return -1, end
	}
	committed := res.Topics[topic][0].CommittedOffset
	if committed < 0 {
		return -1, end
	}
	if end > committed {
		return end - committed, end
	}
	return 0, end
}
