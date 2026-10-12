// p5perf · S11-02 性能复测（NFR-PRF HTTP 面；IoT 面 PRF-002/003/004 由 iotsmoke 计时覆盖，容量面由 devicesim 承担）。
//
// 用例（阈值 01 §6.2 NFR-PRF）：
//
//	api   NFR-PRF-001 列表/详情查询 P95 < 1s（admin-bff orders/devices/reports 轮换）
//	order NFR-PRF-005 下单接口 P95 < 500ms（BFF POST /orders，独立 SKU/库存）
//	poll  NFR-PRF-006 轮询增量接口 P95 < 100ms 且单响应 < 5KB（mobile-bff，500 并发 30s）
//
// 前置（同 p5smoke 口径）：compose core 全绿 + source tools/devenv.sh +
// 起 identity/party/catalog/inventory/order/ops/notification/admin-bff(:8889)/mobile-bff(:8890)。
// 用法：go run ./tools/p5perf [-case all|api|order|poll] [-c 500] [-dur 30s]
// 纪律：严禁并发执行（E1，共享 FIFO 库存池）；种子全 runID 前缀。
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	cpb "micro-server/services/catalog/pb"
	invpb "micro-server/services/inventory/pb"

	"micro-server/tools/internal/devcfg"
)

const (
	seedUID   = 9000000000000000002
	seedAdmin = "13800000000"
	adminBFF  = "http://127.0.0.1:8889/api/v1"
	mobileBFF = "http://127.0.0.1:8890/api/v1"
)

var (
	catalogRPC   = envOr("CATALOG_RPC", "127.0.0.1:8083")
	inventoryRPC = envOr("INVENTORY_RPC", "127.0.0.1:8084")
	runID        = fmt.Sprintf("pf-%d", time.Now().UnixMilli()%100000000)
	concurrency  = flag.Int("c", 500, "poll 用例并发数")
	duration     = flag.Duration("dur", 30*time.Second, "poll 用例时长")
	caseSel      = flag.String("case", "all", "用例 all|api|order|poll")
)

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// ---- 输出 ----

func pass(format string, a ...any) { fmt.Printf("  ✓ "+format+"\n", a...) }

func fail(format string, a ...any) {
	fmt.Printf("  ✗ "+format+"\n", a...)
	fmt.Println("p5perf: FAIL")
	os.Exit(1)
}

func must(err error, what string) {
	if err != nil {
		fail("%s: %v", what, err)
	}
}

type benchResult struct {
	Name    string
	N       int
	Errs    int
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
	Max     time.Duration
	RPS     float64
	MaxBody int
}

func (r benchResult) report() string {
	body := ""
	if r.MaxBody > 0 {
		body = fmt.Sprintf(" 最大响应=%.1fKB", float64(r.MaxBody)/1024)
	}
	return fmt.Sprintf("%s: n=%d errs=%d P50=%s P95=%s P99=%s max=%s RPS=%.0f%s",
		r.Name, r.N, r.Errs, r.P50.Round(time.Millisecond), r.P95.Round(time.Millisecond),
		r.P99.Round(time.Millisecond), r.Max.Round(time.Millisecond), r.RPS, body)
}

// ---- HTTP ----

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type tokens struct {
	AccessToken string `json:"access_token"`
}

func do(base, method, path, token string, body any) (int, envelope, int) {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, rd)
	must(err, "构造请求 "+path)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	must(err, "请求 "+path)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env, len(raw)
}

func mustOK(base, method, path, token string, body any) json.RawMessage {
	st, env, _ := do(base, method, path, token, body)
	if st != http.StatusOK || env.Code != 0 {
		fail("%s %s 期望成功，实际 http=%d code=%d msg=%s", method, path, st, env.Code, env.Msg)
	}
	return env.Data
}

// perfClient 压测专用 HTTP 客户端：DefaultClient 的 MaxIdleConnsPerHost=2 会让
// 高并发每请求重建连接（P95 假性超标——客户端瓶颈非服务端，S11-02 实测）。
var perfClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        4096,
		MaxIdleConnsPerHost: 1024,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     90 * time.Second,
	},
}

// errDo 压测用请求：错误返回不计 exit（网络错误计入 Errs 样例）。
func errDo(base, method, path, token string, body any) (int, envelope, int, error) {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, base+path, rd)
	if err != nil {
		return 0, envelope{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := perfClient.Do(req)
	if err != nil {
		return 0, envelope{}, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env, len(raw), nil
}

// bench 压测引擎：并发 worker 跑满 duration（dur>0）或各跑 n 次（dur=0）。
// paths 轮换请求（GET，token 可空）；maxBody 记录最大响应字节数。
// 加压是 10% 时长的 ramp-up（真实负载不会同毫秒突发；瞬间全量并发会打爆
// dev Windows 监听 backlog 造成 connect refused 假失败——S11-02 实测）。
// 网络错误计入 Errs 并留样例（限 3 条），不中断压测。
func bench(name string, paths []string, token string, c int, dur time.Duration, n int) benchResult {
	var mu sync.Mutex
	lat := []time.Duration{}
	errs, total, maxBody := 0, 0, 0
	var wg sync.WaitGroup
	deadline := time.Now().Add(dur)
	start := time.Now()
	perN := n / len(paths)
	if perN < 1 {
		perN = 1
	}
	ramp := dur / 10
	if n > 0 {
		ramp = 0
	}
	for w := 0; w < c; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			if ramp > 0 {
				time.Sleep(ramp * time.Duration(w) / time.Duration(c))
			}
			i := 0
			for {
				if dur > 0 && time.Now().After(deadline) {
					return
				}
				if dur == 0 && i >= perN {
					return
				}
				path := paths[i%len(paths)]
				t0 := time.Now()
				st, env, blen, derr := errDo(adminBFFBase(path), "GET", path, token, nil)
				el := time.Since(t0)
				ok := derr == nil && st == http.StatusOK && env.Code == 0
				mu.Lock()
				total++
				if derr == nil {
					lat = append(lat, el)
					if blen > maxBody {
						maxBody = blen
					}
				}
				if !ok {
					errs++
					if errs <= 3 {
						fmt.Printf("  · 失败样例：path=%s err=%v http=%d code=%d msg=%s\n", path, derr, st, env.Code, env.Msg)
					}
				}
				mu.Unlock()
				i++
				_ = w
			}
		}(w)
	}
	wg.Wait()
	el := time.Since(start)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	pct := func(p float64) time.Duration {
		if len(lat) == 0 {
			return 0
		}
		return lat[int(float64(len(lat)-1)*p)]
	}
	return benchResult{
		Name: name, N: total, Errs: errs,
		P50: pct(0.50), P95: pct(0.95), P99: pct(0.99), Max: lat[len(lat)-1],
		RPS: float64(total) / el.Seconds(), MaxBody: maxBody,
	}
}

// adminBFFBase poll 用例打 mobile-bff，api 用例打 admin-bff——按路径前缀路由。
func adminBFFBase(path string) string {
	if strings.HasPrefix(path, "/app/") || strings.HasPrefix(path, "/customer/") {
		return mobileBFF
	}
	return adminBFF
}

func main() {
	flag.Parse()
	fmt.Printf("=== p5perf · S11-02 性能复测 runID=%s（严禁并发 E1；c=%d dur=%s） ===\n", runID, *concurrency, *duration)

	adminPW := devcfg.Get("MICRO_ADMIN_PW", "")
	if adminPW == "" {
		fail("MICRO_ADMIN_PW 未注入（source tools/devenv.sh）")
	}
	tid := mustDefaultTenant()
	fmt.Printf("[0] default 租户 = %d\n", tid)
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tid), seedUID)

	catalog := cpb.NewCatalogClient(rpcConn(catalogRPC))
	inv := invpb.NewInventoryClient(rpcConn(inventoryRPC))

	var login struct {
		Tokens tokens `json:"tokens"`
	}
	data := mustOK(adminBFF, "POST", "/auth/login", "", map[string]string{"mobile": seedAdmin, "password": adminPW})
	must(json.Unmarshal(data, &login), "解析 admin token")
	adminTok := login.Tokens.AccessToken

	// ---- mobile token（MOCKWX 登录链：首登建影子账号 → 绑手机签发会话）----
	wxCode := runID + "-wx"
	data = mustOK(mobileBFF, "POST", "/auth/wx-login", "", map[string]string{"code": wxCode})
	var wxFirst struct {
		NeedBindPhone bool `json:"need_bind_phone"`
	}
	must(json.Unmarshal(data, &wxFirst), "解析 wx-login(首登)")
	if !wxFirst.NeedBindPhone {
		fail("首登应 need_bind_phone=true（影子账号未签发 token）")
	}
	var wx struct {
		Tokens *tokens `json:"tokens"`
	}
	data = mustOK(mobileBFF, "POST", "/auth/wx-bind-phone", "", map[string]string{"code": wxCode, "phone": perfPhone()})
	must(json.Unmarshal(data, &wx), "解析 wx-bind-phone")
	if wx.Tokens == nil || wx.Tokens.AccessToken == "" {
		fail("绑定手机号后应签发 token")
	}
	mobileTok := wx.Tokens.AccessToken
	pass("[0] admin + mobile 双端 token 就绪")

	// ---- 种子：party + 压测 SKU（订单用例独立库存 1000）----
	pr, err := catalog.CreateProduct(ctx, &cpb.CreateProductReq{Name: "pf-product-" + runID, Category: "PERF"})
	must(err, "CreateProduct")
	skr, err := catalog.CreateSKU(ctx, &cpb.CreateSKUReq{ProductId: pr.ProductId, Code: "PF-" + runID, Name: "perf-sku", Type: "STANDARD"})
	must(err, "CreateSKU")
	_, err = catalog.SetPrice(ctx, &cpb.SetPriceReq{SkuId: skr.SkuId, PriceType: "RETAIL", Amount: "1.00"})
	must(err, "SetPrice")
	wh, err := inv.CreateWarehouse(ctx, &invpb.CreateWarehouseReq{Code: "PF-WH-" + runID, Name: "perf仓"})
	must(err, "CreateWarehouse")
	_, err = inv.StockIn(ctx, &invpb.StockInReq{WarehouseId: wh.WarehouseId, SkuId: skr.SkuId, Qty: 1000, BizType: "STOCK_IN", BizNo: "PF-IN-" + runID})
	must(err, "StockIn")
	pass("[0] 种子就绪（SKU/仓库/库存1000）")

	var results []benchResult
	if *caseSel == "all" || *caseSel == "api" {
		fmt.Println("\n—— NFR-PRF-001 列表/详情查询（P95 < 1s）——")
		r := bench("api 列表轮换",
			[]string{"/orders?page=1&size=10", "/devices?page=1&size=10", "/reports/sales?days=7", "/warehouses?page=1&size=10"},
			adminTok, 20, 0, 200)
		fmt.Println("  " + r.report())
		if r.P95 >= time.Second {
			fail("NFR-PRF-001 不达标：P95=%s ≥ 1s", r.P95)
		}
		pass("NFR-PRF-001 达标（P95=%s < 1s）", r.P95.Round(time.Millisecond))
		results = append(results, r)
	}
	if *caseSel == "all" || *caseSel == "order" {
		fmt.Println("\n—— NFR-PRF-005 下单接口（P95 < 500ms）——")
		r := benchOrder(adminTok, skr.SkuId, wh.WarehouseId, 100)
		fmt.Println("  " + r.report())
		if r.P95 >= 500*time.Millisecond {
			fail("NFR-PRF-005 不达标：P95=%s ≥ 500ms", r.P95)
		}
		pass("NFR-PRF-005 达标（P95=%s < 500ms，建单 n=%d）", r.P95.Round(time.Millisecond), r.N)
		results = append(results, r)
	}
	if *caseSel == "all" || *caseSel == "poll" {
		fmt.Println("\n—— NFR-PRF-006 轮询增量接口（P95 < 100ms 且 < 5KB；500 并发）——")
		r := bench("poll 轮询轮换",
			[]string{"/app/alerts/pending", "/app/tickets/pending"},
			mobileTok, *concurrency, *duration, 0)
		fmt.Println("  " + r.report())
		if r.P95 >= 100*time.Millisecond {
			fail("NFR-PRF-006 不达标：P95=%s ≥ 100ms", r.P95)
		}
		if r.MaxBody > 5*1024 {
			fail("NFR-PRF-006 不达标：最大响应 %dB > 5KB", r.MaxBody)
		}
		pass("NFR-PRF-006 达标（P95=%s < 100ms，max=%dB < 5KB，%d 并发 RPS=%.0f）",
			r.P95.Round(time.Millisecond), r.MaxBody, *concurrency, r.RPS)
		results = append(results, r)
	}

	fmt.Println("\n—— NFR 复测汇总 ——")
	for _, r := range results {
		fmt.Println("  " + r.report())
	}
	fmt.Println("  IoT 面（PRF-002 遥测端到端 / PRF-003 告警可见性 / PRF-004 指令往返）：go run ./tools/iotsmoke 计时覆盖")
	fmt.Printf("\np5perf: PASS（runID=%s）\n", runID)
}

// benchOrder 下单压测（POST /orders；每单独立 SN 防重）。
func benchOrder(token string, skuID, whID int64, n int) benchResult {
	sku := fmt.Sprintf("%d", skuID)
	wh := fmt.Sprintf("%d", whID)
	var mu sync.Mutex
	lat := []time.Duration{}
	errs, okN := 0, 0
	var wg sync.WaitGroup
	start := time.Now()
	const c = 20
	for w := 0; w < c; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < n; i += c {
				body := map[string]any{
					"type": "DEVICE",
					"items": []map[string]any{{
						"sku_id": sku, "warehouse_id": wh, "qty": 1,
						"sn": fmt.Sprintf("PF-SN-%s-%d", runID, i),
					}},
				}
				t0 := time.Now()
				st, env, _ := do(adminBFF, "POST", "/orders", token, body)
				el := time.Since(t0)
				mu.Lock()
				lat = append(lat, el)
				if st == http.StatusOK && env.Code == 0 {
					okN++
				} else {
					errs++
					if errs <= 3 {
						fmt.Printf("  · 下单失败样例 http=%d code=%d msg=%s\n", st, env.Code, env.Msg)
					}
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	el := time.Since(start)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	pct := func(p float64) time.Duration { return lat[int(float64(len(lat)-1)*p)] }
	return benchResult{
		Name: "order 下单", N: okN, Errs: errs,
		P50: pct(0.50), P95: pct(0.95), P99: pct(0.99), Max: lat[len(lat)-1],
		RPS: float64(okN) / el.Seconds(),
	}
}

func rpcConn(addr string) grpc.ClientConnInterface {
	conn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{addr},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
	return conn.Conn()
}

// perfPhone runID 派生手机号（mobile_hash 全局 UK，跨 run 不重复）。
func perfPhone() string {
	var seed int64
	_, _ = fmt.Sscanf(strings.TrimPrefix(runID, "pf-"), "%d", &seed)
	return fmt.Sprintf("136%08d", seed%100000000)
}

func mustDefaultTenant() int64 {
	dsn := devcfg.Get("MICRO_DSN_IDENTITY", devcfg.Get("DSN", ""))
	if dsn == "" {
		fail("MICRO_DSN_IDENTITY/DSN 未注入")
	}
	db, err := sql.Open("mysql", dsn)
	must(err, "连 identity 库")
	defer func() { _ = db.Close() }()
	var tid int64
	must(db.QueryRow("select tenant_id from tenant where tenant_code='default'").Scan(&tid), "查 default 租户")
	return tid
}
