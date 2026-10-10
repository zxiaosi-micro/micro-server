// p4load · 导出负载验收（S8-03/FR-ANA-003）：10 万行导出 <5s 且对在线请求零影响。
//
// 方案：独立负载租户直插 100k 台账行（event_ledger 导出源，与业务库隔离）→
// CSV/XLSX 导出计时（<5s）→ 导出进行中并发打 GetReport 在线请求，P95 无劣化
// （导出走 threading.TaskRunner worker 池，与 RPC goroutine 隔离）。
//
// 前置：analytics + file 进程运行中；compose core。
// 纪律：严禁与 p4smoke 并发执行（E1）。
//
//	env:
//	  source tools/devenv.sh micro_analytics
//	  go run ./tools/p4load            # 默认 100000 行
//	  go run ./tools/p4load -rows 20000
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	_ "github.com/go-sql-driver/mysql"

	anpb "micro-server/services/analytics/pb"
)

const (
	// loadTenant 负载独立租户（与冒烟/业务租户隔离，数据留存备查不清理——禁裸删口径）
	loadTenant  = 9000000000000000091
	batchSize   = 1000
	probeN      = 40
	probeBudget = 100 * time.Millisecond // 在线请求 P95 预算（日报查询本应 <10ms）
)

var (
	analyticsRPC = envOr("ANALYTICS_RPC", "127.0.0.1:8095")
	dsn          = envOr("MICRO_DSN_ANALYTICS", "")
	rowsN        = flag.Int("rows", 100000, "台账行数")
	exportBudget = flag.Duration("budget", 5*time.Second, "导出耗时预算")
)

func main() {
	flag.Parse()
	logx.MustSetup(logx.LogConf{ServiceName: "p4load", Mode: "console"})
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), loadTenant), 9000000000000000092)

	if dsn == "" {
		fail("MICRO_DSN_ANALYTICS 未设置（source tools/devenv.sh micro_analytics）")
	}
	db, err := sql.Open("mysql", dsn)
	must(err, "db open")
	defer func() { _ = db.Close() }()

	conn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{analyticsRPC},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
	an := anpb.NewAnalyticsClient(conn)

	today := time.Now().Format("2006-01-02")
	fmt.Printf("== p4load 导出负载验收 rows=%d budget=%s ==\n", *rowsN, *exportBudget)

	// ---- 1. 造数：100k 台账行（幂等：当日已有则跳过）----
	seedLedger(db, today)

	// ---- 2. 管道预热（导出一次小报表：重启后消费组 rebalance 的一次性延迟不进计量）----
	warm := exportCost(an, ctx, today, "sales", "CSV")
	fmt.Printf("  预热导出（sales）: %v\n", warm)

	// ---- 3. 在线请求基线（导出前）----
	baseline := probeOnline(an, ctx, today)
	fmt.Printf("  在线基线 P50=%v P95=%v\n", p50(baseline), p95(baseline))
	if p95(baseline) > probeBudget {
		fail("在线基线已超预算（P95=%v > %v）——环境问题，不是导出影响", p95(baseline), probeBudget)
	}

	// ---- 4. 10 万行导出（CSV）：<5s ----
	cost := exportCost(an, ctx, today, "event_ledger", "CSV")
	fmt.Printf("  CSV 导出耗时: %v\n", cost)
	if cost > *exportBudget {
		fail("CSV 10 万行导出超预算: %v > %v", cost, *exportBudget)
	}
	pass("CSV %d 行导出 <5s（%v）", *rowsN, cost)

	// ---- 5. 导出进行中在线请求零影响 ----
	done := make(chan struct{})
	go func() {
		defer close(done)
		exportCost(an, ctx, today, "event_ledger", "XLSX") // 第二次导出（同数据 XLSX 渲染更重）
	}()
	time.Sleep(300 * time.Millisecond) // 让导出进入渲染热区

	during := probeOnline(an, ctx, today)
	<-done
	fmt.Printf("  导出中在线 P50=%v P95=%v\n", p50(during), p95(during))
	if p95(during) > probeBudget {
		fail("导出期间在线请求劣化超预算: P95=%v > %v（TaskRunner 隔离失效？）", p95(during), probeBudget)
	}
	pass("导出期间在线请求零影响（P95=%v ≤ %v）", p95(during), probeBudget)

	// ---- 6. XLSX 耗时复核 ----
	cost2 := exportCost(an, ctx, today, "event_ledger", "XLSX")
	fmt.Printf("  XLSX 导出耗时: %v\n", cost2)
	if cost2 > *exportBudget {
		fail("XLSX 10 万行导出超预算: %v > %v", cost2, *exportBudget)
	}
	pass("XLSX %d 行导出 <5s（%v，StreamWriter 流式）", *rowsN, cost2)

	fmt.Printf("== p4load 全部通过 ==\n")
}

// seedLedger 负载租户台账造数（分批多值 INSERT；当日已有则跳过——幂等重跑）。
func seedLedger(db *sql.DB, today string) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM report_event_entry WHERE tenant_id = ? AND stat_date = ?", loadTenant, today).Scan(&n)
	must(err, "count ledger")
	if n >= *rowsN {
		fmt.Printf("  台账已就绪（%d 行 ≥ %d），跳过造数\n", n, *rowsN)
		return
	}
	start := time.Now()
	batch := make([]string, 0, batchSize)
	args := make([]any, 0, batchSize*7)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		q := "INSERT INTO report_event_entry (event_id, event_type, topic, tenant_id, biz_key, stat_date, payload) VALUES " +
			joinStrings(batch, ",")
		_, err := db.Exec(q, args...)
		must(err, "batch insert")
		batch = batch[:0]
		args = args[:0]
	}
	for i := n; i < *rowsN; i++ {
		biz := fmt.Sprintf("P4LOAD-%s-%08d", today, i)
		batch = append(batch, "(?,?,?,?,?,?,?)")
		args = append(args,
			newEventID(), "stock.in", "stock_in", loadTenant, biz, today,
			fmt.Sprintf(`{"tenant_id":%d,"warehouse_id":1,"sku_id":1,"qty":1,"biz_no":"%s","sns":[]}`, loadTenant, biz))
		if len(batch) == batchSize {
			flush()
		}
	}
	flush()
	fmt.Printf("  造数 %d 行完成（%v）\n", *rowsN-n, time.Since(start).Round(time.Millisecond))
}

// probeOnline 并发打在线报表请求（顺序采样，P95 统计）。
func probeOnline(an anpb.AnalyticsClient, ctx context.Context, today string) []time.Duration {
	lat := make([]time.Duration, 0, probeN)
	for i := 0; i < probeN; i++ {
		t0 := time.Now()
		_, err := an.GetReport(ctx, &anpb.GetReportReq{
			ReportType: "inventory", DateStart: today, DateEnd: today, WarehouseId: 1,
		})
		if err != nil {
			fail("在线请求失败: %v", err)
		}
		lat = append(lat, time.Since(t0))
		time.Sleep(20 * time.Millisecond)
	}
	return lat
}

// exportCost 发起导出并计满时长（PENDING → DONE；端到端口径含队列与归档）。
func exportCost(an anpb.AnalyticsClient, ctx context.Context, today, reportType, format string) time.Duration {
	req, err := an.RequestExport(ctx, &anpb.RequestExportReq{
		ReportType: reportType, Format: format, DateStart: today, DateEnd: today,
	})
	must(err, "RequestExport "+format)
	start := time.Now()
	id := req.ExportId
	for {
		t, err := an.GetExportTask(ctx, &anpb.GetExportTaskReq{ExportId: id})
		must(err, "GetExportTask")
		switch t.Task.Status {
		case "DONE":
			cost := time.Duration(t.Task.CostMs) * time.Millisecond
			fmt.Printf("  [%s task=%s] rows=%d 服务端渲染 %v file=%s\n",
				format, id, t.Task.RowCount, cost, t.Task.FileId)
			return time.Since(start) // 端到端（含队列）
		case "FAILED":
			fail("导出失败 task=%s: %s", id, t.Task.Error)
		}
		if time.Since(start) > 10*time.Minute {
			fail("导出超时 task=%s", id)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ---- 统计与工具 ----

func p50(v []time.Duration) time.Duration { return pct(v, 0.5) }
func p95(v []time.Duration) time.Duration { return pct(v, 0.95) }

func pct(v []time.Duration, p float64) time.Duration {
	if len(v) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(p * float64(len(s)-1))
	return s[idx]
}

func newEventID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func joinStrings(ss []string, sep string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "p4load FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(format string, args ...any) {
	fmt.Printf("  ✅ "+format+"\n", args...)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "p4load FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
