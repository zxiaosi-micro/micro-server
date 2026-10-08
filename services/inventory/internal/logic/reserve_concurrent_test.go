// 防超卖并发固化用例（S4-03，CI 必跑）：
//
//	100 goroutine 抢 50 库存 → 恰好 50 成功、50 拒绝，余量不为负、零超卖（02 §9.2）。
//	同时固化：Redis 网关路径 + 纯 DB 降级路径 + 幂等重放 1062 → ErrTxnReplay + 流水核对。
//
// 环境三级解析走 micro-common/testinfra（env → 本机 dev → container；全部不可用 skip）。

package logic

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"github.com/zxiaosi-micro/micro-common/tenantx"
	"github.com/zxiaosi-micro/micro-common/testinfra"
)

const (
	concTotal   = 100 // 并发 goroutine 数（任务口径：100 抢 50）
	concStock   = 50  // 初始库存
	testTenant  = 900000000000000001
	testSkuBase = int64(890000000000000001)
)

// testSnowflake 进程级共享雪花节点——同 worker 的多个节点在同毫秒内会撞 ID，
// 必须全测试二进制共用一个（并发测试与幂等测试都用它发号）。
var (
	testSnowflakeOnce sync.Once
	testSnowflakeNode *snowflake.Node
	testSnowflakeDone func()
	testSnowflakeErr  error
)

func testSnowflake() (*snowflake.Node, func(), error) {
	testSnowflakeOnce.Do(func() {
		if os.Getenv("MICRO_WORKER_ID") == "" {
			_ = os.Setenv("MICRO_WORKER_ID", "77")
		}
		testSnowflakeNode, testSnowflakeDone, testSnowflakeErr = snowflake.NewAuto(context.Background(), nil)
	})
	return testSnowflakeNode, testSnowflakeDone, testSnowflakeErr
}

// newTestServiceCtx 组装直连测试库的 ServiceContext（不走 etcd 雪花，用 MICRO_WORKER_ID）。
func newTestServiceCtx(t *testing.T, dsn string, withRedis bool) *svc.ServiceContext {
	t.Helper()

	node, cancel, err := testSnowflake()
	if err != nil {
		t.Fatalf("snowflake: %v", err)
	}
	t.Cleanup(func() { cancel() })

	// 模型行缓存节点（生成物 FindOne 需要；测试不走行缓存路径但构造不能为空）
	rc, _ := testinfra.Redis(t)
	cacheConf := cache.CacheConf{{
		RedisConf: redis.RedisConf{Host: rc.Addr, Pass: rc.Password, Type: "node"},
		Weight:    100,
	}}

	var rd *redis.Redis
	if withRedis {
		rd = redis.MustNewRedis(redis.RedisConf{Host: rc.Addr, Pass: rc.Password, Type: "node"})
	}

	conn := sqlx.NewMysql(dsn)
	return &svc.ServiceContext{
		Conn: conn,
		Models: &svc.Models{
			Warehouse:     model.NewWarehouseModel(conn, cacheConf),
			Inventory:     model.NewInventoryModel(conn, cacheConf),
			StockRecord:   model.NewStockRecordModel(conn, cacheConf),
			Stocktake:     model.NewStocktakeModel(conn, cacheConf),
			StocktakeItem: model.NewStocktakeItemModel(conn, cacheConf),
		},
		Rd:        rd,
		Snowflake: node,
	}
}

// setupTestDB 建独立测试库并应用迁移（multiStatements 一次执行全文件）。
// DSN 库名段重写但保留全部查询参数（parseTime 等必须保留，否则 time.Time 扫描失败）。
func setupTestDB(t *testing.T) string {
	t.Helper()
	adminDsn, _ := testinfra.MySQL(t)

	admin, err := sql.Open("mysql", adminDsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec("DROP DATABASE IF EXISTS micro_inventory_test;" +
		"CREATE DATABASE micro_inventory_test DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatalf("create db: %v", err)
	}

	raw, err := os.ReadFile("../../migrations/000001_init.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	testDsn := withDBName(adminDsn, "micro_inventory_test") + "&multiStatements=true"
	admin, err = sql.Open("mysql", testDsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(string(raw)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return withDBName(adminDsn, "micro_inventory_test")
}

// withDBName 重写 DSN 库名段（"…/db?params" / "…/?params"）。
func withDBName(dsn, name string) string {
	base := dsn[:strings.LastIndex(dsn, "/")+1]
	rest := dsn[len(base):] // "db?params" 或 "?params"
	params := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		params = rest[i:] // 含 "?"
	}
	if params == "" {
		return base + name
	}
	return base + name + params
}

// seedStock 建仓 + 入库 qty 并初始化 Redis 计数器。
func seedStock(t *testing.T, sc *svc.ServiceContext, wid, skuId, qty int64) {
	t.Helper()
	ctx := context.Background()

	if _, err := sc.Models.Warehouse.Insert(ctx, &model.Warehouse{
		WarehouseId: wid,
		Code:        "WH-CONC-" + fmt.Sprint(wid%100000),
		Name:        "并发测试仓",
		Status:      1,
		TenantId:    testTenant,
	}); err != nil {
		if !strings.Contains(err.Error(), "1062") {
			t.Fatalf("insert warehouse: %v", err)
		}
	}
	if err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return sc.Models.Inventory.InitRowOnDupInTx(ctx, session, &model.Inventory{
			InventoryId: sc.Snowflake.MustNextID(),
			WarehouseId: wid,
			SkuId:       skuId,
			Available:   qty,
			TenantId:    testTenant,
		})
	}); err != nil {
		t.Fatalf("init inventory: %v", err)
	}
	if sc.Rd != nil {
		if err := sc.Rd.SetCtx(ctx, redisKey(testTenant, wid, skuId), fmt.Sprint(qty)); err != nil {
			t.Fatalf("seed redis gate: %v", err)
		}
	}
}

func assertFinalState(t *testing.T, sc *svc.ServiceContext, wid, skuId int64, wantLocked int64) {
	t.Helper()
	inv, err := sc.Models.Inventory.FindOne(context.Background(), testTenant, wid, skuId)
	if err != nil {
		t.Fatalf("final FindOne: %v", err)
	}
	if inv.Available != 0 {
		t.Fatalf("零超卖破坏：available=%d 期望 0", inv.Available)
	}
	if inv.Available < 0 || inv.Locked < 0 {
		t.Fatalf("负库存：available=%d locked=%d", inv.Available, inv.Locked)
	}
	if wantLocked > 0 && inv.Locked != wantLocked {
		t.Fatalf("locked=%d 期望 %d", inv.Locked, wantLocked)
	}
}

// TestReserveConcurrency100For50 CI 固化：100 goroutine 抢 50，零超卖（Redis 网关路径）。
func TestReserveConcurrency100For50(t *testing.T) {
	dsn := setupTestDB(t)
	sc := newTestServiceCtx(t, dsn, true)
	if sc.Rd == nil {
		t.Skip("testinfra: Redis 不可用，跳过网关路径（纯 DB 路径由下一用例覆盖）")
	}
	// 注入租户上下文（业务面 fail-closed 口径，02 §9.4）
	ctx := tenantx.WithTenant(context.Background(), testTenant)

	wid := int64(770001)
	skuId := testSkuBase
	seedStock(t, sc, wid, skuId, concStock)

	var okCount, failCount int64
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < concTotal; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l := NewReserveLogic(ctx, sc)
			_, err := l.Reserve(&pb.ReserveReq{WarehouseId: wid, SkuId: skuId, Qty: 1, BizType: "ORDER", BizNo: fmt.Sprintf("CONC-%d", n)})
			if err == nil {
				atomic.AddInt64(&okCount, 1)
			} else {
				atomic.AddInt64(&failCount, 1)
			}
		}(i)
	}
	wg.Wait()
	t.Logf("100 抢 50 耗时 %v：成功 %d / 拒绝 %d", time.Since(start), okCount, failCount)

	if okCount != concStock || failCount != concTotal-concStock {
		t.Fatalf("成功 %d / 拒绝 %d，期望 %d / %d", okCount, failCount, concStock, concTotal-concStock)
	}
	assertFinalState(t, sc, wid, skuId, concStock)

	// 流水核对：成功数 = RESERVE 流水数
	records, total, err := sc.Models.StockRecord.ListPage(ctx, testTenant, wid, skuId, "RESERVE", 1, 100)
	if err != nil {
		t.Fatalf("list records: %v", err)
	}
	if total != concStock || len(records) != concStock {
		t.Fatalf("RESERVE 流水 %d 条(len=%d)，期望 %d", total, len(records), concStock)
	}
	for _, r := range records {
		if r.AfterAvailable < 0 {
			t.Fatalf("流水出现负余量：biz_no=%s after=%d", r.BizNo, r.AfterAvailable)
		}
	}
}

// TestReservePureDBPath 纯 DB 降级路径（Rd=nil）同样零超卖（02 §9.2 第 4 步）。
func TestReservePureDBPath(t *testing.T) {
	dsn := setupTestDB(t)
	sc := newTestServiceCtx(t, dsn, false)
	ctx := tenantx.WithTenant(context.Background(), testTenant)

	wid := int64(770002)
	skuId := testSkuBase + 1
	seedStock(t, sc, wid, skuId, concStock)

	var okCount int64
	var wg sync.WaitGroup
	for i := 0; i < concTotal; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l := NewReserveLogic(ctx, sc)
			if _, err := l.Reserve(&pb.ReserveReq{WarehouseId: wid, SkuId: skuId, Qty: 1, BizType: "ORDER", BizNo: fmt.Sprintf("PUREDB-%d", n)}); err == nil {
				atomic.AddInt64(&okCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if okCount != concStock {
		t.Fatalf("纯 DB 路径成功 %d，期望 %d（超卖/误拒均不合格）", okCount, concStock)
	}
	assertFinalState(t, sc, wid, skuId, concStock)
}

// TestReserveIdempotentReplay 幂等键：(biz_type,biz_no) 重复 → ErrTxnReplay，余量不变。
func TestReserveIdempotentReplay(t *testing.T) {
	dsn := setupTestDB(t)
	sc := newTestServiceCtx(t, dsn, false)
	ctx := tenantx.WithTenant(context.Background(), testTenant)

	wid := int64(770003)
	skuId := testSkuBase + 2
	seedStock(t, sc, wid, skuId, 10)

	l := NewReserveLogic(ctx, sc)
	if _, err := l.Reserve(&pb.ReserveReq{WarehouseId: wid, SkuId: skuId, Qty: 3, BizType: "ORDER", BizNo: "REPLAY-1"}); err != nil {
		t.Fatalf("首次预留失败: %v", err)
	}
	_, err := l.Reserve(&pb.ReserveReq{WarehouseId: wid, SkuId: skuId, Qty: 3, BizType: "ORDER", BizNo: "REPLAY-1"})
	if err == nil || err.Error() != ErrTxnReplay.Error() {
		t.Fatalf("重放应返回 ErrTxnReplay，得到 %v", err)
	}
	// 重放不产生副作用：available 仍为 7、locked 仍为 3，流水仅 1 条
	inv, err := sc.Models.Inventory.FindOne(ctx, testTenant, wid, skuId)
	if err != nil {
		t.Fatalf("replay FindOne: %v", err)
	}
	if inv.Available != 7 || inv.Locked != 3 {
		t.Fatalf("重放改变了库存：available=%d locked=%d，期望 7/3", inv.Available, inv.Locked)
	}
	if _, total, rerr := sc.Models.StockRecord.ListPage(ctx, testTenant, wid, skuId, "RESERVE", 1, 100); rerr != nil || total != 1 {
		t.Fatalf("RESERVE 流水应仅 1 条：total=%d err=%v", total, rerr)
	}
}
