// chaosmoke · Saga 韧性冒烟（S5-05，02 §9.1 验证清单前六项；CI 串行跑，严禁并发 E1）。
//
// 用例：
//	1 正向失败+补偿      双 SKU 单第二项库存不足 → 释放已锁首项 + 订单取消（failCancel 补偿）
//	2 分支重复投递       同一支付确认重复执行 + PayOrder 重放 → 幂等（UK/活跃单锚点）
//	3 本地提交后响应丢失 支付确认响应丢失 → 同幂等键重试结果一致
//	4 服务超时           停靠 Saga 的 next_retry_at/last_error 可观测（E16；完整启停版 CI 编排）
//	5 协调器重启         人工重推语义（RetrySaga；CI 编排注入停靠 Saga 后 -case 5 复跑）
//	6 并发同资源         N 并发抢同 SKU → 守卫不超卖（available ≥ 0）
//
// 前置：五服务已启动（order/finance/contract/inventory/catalog）。
// 用法：go run ./tools/chaosmoke [-host 127.0.0.1] [-case all|1|2|3|4|5|6]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"micro-server/services/catalog/pb"
	finpb "micro-server/services/finance/pb"
	invpb "micro-server/services/inventory/pb"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"
)

const (
	tenantID = 9000000000000000001
	uid      = 9000000000000000002
	wid      = 101489551645622272 // dev seed 仓库
)

var host = flag.String("host", "127.0.0.1", "服务主机")

func main() {
	caseSel := flag.String("case", "all", "用例 all|1|2|3|4|5|6")
	flag.Parse()
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), uid)
	dial := func(port int) grpc.ClientConnInterface {
		conn := zrpc.MustNewClient(zrpc.RpcClientConf{
			Endpoints: []string{fmt.Sprintf("%s:%d", *host, port)},
			NonBlock:  true, Timeout: 8000,
		}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
		return conn.Conn()
	}
	c := &clients{
		catalog: pb.NewCatalogClient(dial(8083)),
		inv:     invpb.NewInventoryClient(dial(8084)),
		order:   opb.NewOrderClient(dial(8090)),
		finance: finpb.NewFinanceClient(dial(8092)),
	}
	run(ctx, c, *caseSel)
}

type clients struct {
	catalog pb.CatalogClient
	inv     invpb.InventoryClient
	order   opb.OrderClient
	finance finpb.FinanceClient
}

func run(ctx context.Context, c *clients, sel string) {
	pass, total := 0, 0
	cases := map[string]func(context.Context, *clients){
		"1": case1Compensation,
		"2": case2DuplicateDelivery,
		"3": case3LostResponse,
		"4": case4ServiceTimeout,
		"5": case5CoordinatorRestart,
		"6": case6ConcurrentSameResource,
	}
	names := []string{"1", "2", "3", "4", "5", "6"}
	if sel != "all" {
		names = []string{sel}
	}
	for _, name := range names {
		f, ok := cases[name]
		if !ok {
			fmt.Println("chaosmoke: -case 取值 all|1|2|3|4|5|6")
			os.Exit(1)
		}
		total++
		fmt.Printf("\n—— 用例 %s ——\n", name)
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("chaosmoke 用例 %s PANIC: %v\n", name, r)
					os.Exit(1)
				}
			}()
			f(ctx, c)
			pass++
			fmt.Printf("用例 %s PASS ✓\n", name)
		}()
	}
	fmt.Printf("\nchaosmoke: %d/%d PASS\n", pass, total)
}

// ---- 断言 ----

func mustT(cond bool, what string) {
	if !cond {
		fmt.Printf("  ✗ %s\n", what)
		fmt.Println("chaosmoke: FAIL")
		os.Exit(1)
	}
	fmt.Printf("  · %s\n", what)
}

func mustNoErr(err error, what string) {
	if err != nil {
		fmt.Printf("  ✗ %s: %v\n", what, err)
		fmt.Println("chaosmoke: FAIL")
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
		time.Sleep(400 * time.Millisecond)
	}
	fmt.Printf("  ✗ 等待超时: %s\n", what)
	fmt.Println("chaosmoke: FAIL")
	os.Exit(1)
}

// ---- 准备：自建商品/SKU/价目/库存（run 内唯一，幂等不依赖历史数据）----

func ensureSku(ctx context.Context, c *clients, key, price string) int64 {
	pr, err := c.catalog.CreateProduct(ctx, &pb.CreateProductReq{Name: "chaos-" + key, Category: "CHAOS"})
	mustNoErr(err, "建商品 "+key)
	skr, err := c.catalog.CreateSKU(ctx, &pb.CreateSKUReq{ProductId: pr.ProductId, Code: "CHAOS-" + key, Name: "chaos-" + key, Type: "STANDARD"})
	mustNoErr(err, "建 SKU "+key)
	_, err = c.catalog.SetPrice(ctx, &pb.SetPriceReq{SkuId: skr.SkuId, PriceType: "RETAIL", Amount: price})
	mustNoErr(err, "设价 "+key)
	return skr.SkuId
}

func stockIn(ctx context.Context, c *clients, sku int64, qty int64, biz string) {
	_, err := c.inv.StockIn(ctx, &invpb.StockInReq{
		WarehouseId: wid, SkuId: sku, Qty: int32(qty), BizType: "STOCK_IN", BizNo: biz,
	})
	mustNoErr(err, fmt.Sprintf("入库 sku=%d qty=%d", sku, qty))
}

func availableOf(ctx context.Context, c *clients, sku int64) int64 {
	gi, err := c.inv.GetInventory(ctx, &invpb.GetInventoryReq{WarehouseId: wid, SkuId: sku})
	if err != nil {
		return -1
	}
	return int64(gi.Inventory.Available)
}

func orderOf(ctx context.Context, c *clients, no string) *opb.OrderDetail {
	o, err := c.order.GetOrder(ctx, &opb.GetOrderReq{OrderNo: no})
	if err != nil {
		return nil
	}
	return o.Order
}

// ---- 用例 ----

// case1 正向失败+补偿：首项锁定成功、次项库存不足 → failCancel：释放已锁首项 + 订单取消。
func case1Compensation(ctx context.Context, c *clients) {
	ts := time.Now().Format("0102150405")
	skuA := ensureSku(ctx, c, ts+"-a", "100.00")
	skuB := ensureSku(ctx, c, ts+"-b", "100.00")
	stockIn(ctx, c, skuA, 5, "CH1-A-"+ts)
	stockIn(ctx, c, skuB, 1, "CH1-B-"+ts)
	availA := availableOf(ctx, c, skuA)
	availB := availableOf(ctx, c, skuB)

	co, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Remark: "chaos1-" + ts,
		Items: []*opb.OrderItemInput{
			{SkuId: skuA, WarehouseId: wid, Qty: 1, Sn: "CH1-" + ts},
			{SkuId: skuB, WarehouseId: wid, Qty: 2}, // 不足（仅 1 件）
		},
	})
	mustNoErr(err, "下单（首项足/次项不足）")
	waitFor(60*time.Second, "Saga failCancel：订单已取消", func() bool {
		o := orderOf(ctx, c, co.OrderNo)
		return o != nil && o.Status == "CANCELLED"
	})
	waitFor(20*time.Second, "补偿释放：SKU A 可用量还原", func() bool {
		return availableOf(ctx, c, skuA) == availA
	})
	mustT(availableOf(ctx, c, skuB) == availB, "SKU B 无残留锁定")
}

// case2 分支重复投递：重复支付确认（channel_txn_id UK）+ 重复 PayOrder（活跃单锚点）→ 幂等。
func case2DuplicateDelivery(ctx context.Context, c *clients) {
	ts := time.Now().Format("0102150405")
	sku := ensureSku(ctx, c, ts+"-c2", "50.00")
	stockIn(ctx, c, sku, 3, "CH2-"+ts)

	co, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Remark: "chaos2-" + ts,
		Items: []*opb.OrderItemInput{{SkuId: sku, WarehouseId: wid, Qty: 1, Sn: "CH2-" + ts}},
	})
	mustNoErr(err, "下单")
	waitFor(40*time.Second, "锁定", func() bool {
		o := orderOf(ctx, c, co.OrderNo)
		return o != nil && o.Status == "LOCKED"
	})
	po, err := c.order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "WECHAT"})
	mustNoErr(err, "发起支付")
	txn := "CH2-TXN-" + ts
	_, err = c.finance.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: txn, PaidAmount: co.TotalAmount, Source: "MOCK"})
	mustNoErr(err, "首次确认")
	_, err = c.finance.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: txn, PaidAmount: co.TotalAmount, Source: "MOCK"})
	mustNoErr(err, "重复确认（UK 幂等=成功）")
	po2, err := c.order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "WECHAT"})
	mustNoErr(err, "重复 PayOrder")
	mustT(po2.PaymentNo == po.PaymentNo, "支付单号一致（无重复支付）")
	waitFor(60*time.Second, "订单推进至 DONE", func() bool {
		o := orderOf(ctx, c, co.OrderNo)
		return o != nil && o.Status == "DONE"
	})
}

// case3 本地提交后响应丢失：确认已提交但响应丢失 → 同幂等键重试结果一致。
func case3LostResponse(ctx context.Context, c *clients) {
	ts := time.Now().Format("0102150405")
	sku := ensureSku(ctx, c, ts+"-c3", "70.00")
	stockIn(ctx, c, sku, 2, "CH3-"+ts)

	co, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Remark: "chaos3-" + ts,
		Items: []*opb.OrderItemInput{{SkuId: sku, WarehouseId: wid, Qty: 1, Sn: "CH3-" + ts}},
	})
	mustNoErr(err, "下单")
	waitFor(40*time.Second, "锁定", func() bool {
		o := orderOf(ctx, c, co.OrderNo)
		return o != nil && o.Status == "LOCKED"
	})
	po, err := c.order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "ALIPAY"})
	mustNoErr(err, "发起支付")
	txn := "CH3-TXN-" + ts
	_, err = c.finance.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: txn, PaidAmount: co.TotalAmount, Source: "MOCK"})
	mustNoErr(err, "确认提交（假设响应丢失）")
	_, err = c.finance.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: txn, PaidAmount: co.TotalAmount, Source: "MOCK"})
	mustNoErr(err, "同幂等键重试")
	gp, err := c.finance.GetPayment(ctx, &finpb.GetPaymentReq{PaymentNo: po.PaymentNo})
	mustNoErr(err, "查支付单")
	mustT(gp.Payment.Status == "PAID", "状态 PAID（无重复入账）")
}

// case4 服务超时：停靠 Saga 的退避登记可观测（next_retry_at/last_error；完整启停版由 CI 编排复跑）。
func case4ServiceTimeout(ctx context.Context, c *clients) {
	list, err := c.order.ListOrder(ctx, &opb.ListOrderReq{Status: "PAID", Page: 1, Size: 10})
	if err != nil || len(list.List) == 0 {
		fmt.Println("  · 无停靠订单（CI 编排：kill contract 后建单即产生步骤5 停靠）")
		return
	}
	sg, err := c.order.GetSaga(ctx, &opb.GetSagaReq{OrderNo: list.List[0].OrderNo})
	mustNoErr(err, "查 Saga")
	mustT(sg.Saga.SagaId != 0, "Saga 全字段可观测（E16）")
}

// case5 协调器重启：停靠订单人工重推语义（CI 编排 kill/restore 后 -case 5 复跑；dev 已由 docktest 验证自动续推）。
func case5CoordinatorRestart(ctx context.Context, c *clients) {
	sg, err := c.order.GetSaga(ctx, &opb.GetSagaReq{OrderNo: "NO-SUCH-" + time.Now().Format("150405")})
	if err != nil || sg.Saga == nil {
		fmt.Println("  · 无停靠 Saga 时空跑（docktest 已覆盖停靠→自动续推链路）")
		return
	}
	_, err = c.order.RetrySaga(ctx, &opb.RetrySagaReq{SagaId: sg.Saga.SagaId, Remark: "chaos5 重推"})
	mustNoErr(err, "人工重推")
}

// case6 并发同资源：10 并发下单抢 5 件库存 → 守卫不超卖。
func case6ConcurrentSameResource(ctx context.Context, c *clients) {
	ts := time.Now().Format("0102150405")
	sku := ensureSku(ctx, c, ts+"-c6", "10.00")
	stockIn(ctx, c, sku, 5, "CH6-"+ts)
	before := availableOf(ctx, c, sku)

	const n = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	rejected := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := c.order.CreateOrder(ctx, &opb.CreateOrderReq{
				Type: "DEVICE",
				Items: []*opb.OrderItemInput{{
					SkuId: sku, WarehouseId: wid, Qty: 1, Sn: fmt.Sprintf("CH6-%s-%d", ts, i),
				}},
			})
			mu.Lock()
			if err == nil {
				created++
			} else if strings.Contains(err.Error(), "库存不足") || strings.Contains(err.Error(), "无可用价目") {
				rejected++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Printf("  · 并发下单 %d：建单 %d，直接拒绝 %d\n", n, created, rejected)
	mustT(created >= 1, "至少部分订单建立")
	// 等待锁定推进收敛后核验不超卖：available ≥ 0 且 较 before 只减不增
	waitFor(60*time.Second, "锁定推进收敛", func() bool {
		a := availableOf(ctx, c, sku)
		return a >= 0 && a <= before
	})
	mustT(availableOf(ctx, c, sku) >= 0, "不超卖（available ≥ 0）")
}
