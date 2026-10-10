// e2esmoke · 交易域全链冒烟（S5-05）：导入 SN→入库→下单→支付→Saga→出库→签收→质保起算。
//
// 前置：order/finance/contract/inventory/catalog 五服务已启动（dev 端口 8090/8092/8093/8084/8083），
// Kafka + 各服务 Outbox Relay/消费在跑（事件驱动推进）。
// 严禁并发执行（共享库存池，E1）；跑前 go run ./tools/mqinit。
//
// 用法：go run ./tools/e2esmoke [-host 127.0.0.1]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"micro-server/services/catalog/pb"
	finpb "micro-server/services/finance/pb"
	ctpb "micro-server/services/contract/pb"
	invpb "micro-server/services/inventory/pb"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"
)

// seed 口径（tools/seed 固定 ID）：租户 9000000000000000001，管理员 9000000000000000002。
const (
	tenantID = 9000000000000000001
	uid      = 9000000000000000002
)

func main() {
	host := flag.String("host", "127.0.0.1", "服务主机")
	timeout := flag.Int("timeout", 90, "Saga 推进等待秒数")
	flag.Parse()

	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), uid)
	dial := func(port int) grpc.ClientConnInterface {
		conn := zrpc.MustNewClient(zrpc.RpcClientConf{
			Endpoints: []string{fmt.Sprintf("%s:%d", *host, port)},
			NonBlock:  true,
			Timeout:   8000,
		}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
		return conn.Conn()
	}
	catalog := pb.NewCatalogClient(dial(8083))
	inv := invpb.NewInventoryClient(dial(8084))
	order := opb.NewOrderClient(dial(8090))
	fin := finpb.NewFinanceClient(dial(8092))
	contract := ctpb.NewContractClient(dial(8093))

	ts := time.Now().Format("0102T150405")
	sn := "SMOKE" + ts
	biz := "SMOKE" + ts
	pass := func(step string) { fmt.Printf("  ✓ %s\n", step) }
	fail := func(step string, err error) {
		fmt.Printf("  ✗ %s: %v\n", step, err)
		fmt.Println("e2esmoke: FAIL（前置检查：五服务已启动？mqinit 已跑？Kafka Relay 在跑？）")
		os.Exit(1)
	}

	fmt.Println("e2esmoke 全链：导入 SN→入库→下单→支付→Saga→出库→签收→质保")

	// 1. 商品 + SKU + 价格（时间戳编码，run 内唯一）
	pr, err := catalog.CreateProduct(ctx, &pb.CreateProductReq{Name: "冒烟商品" + ts, Category: "SMOKE"})
	if err != nil {
		fail("建商品", err)
	}
	skr, err := catalog.CreateSKU(ctx, &pb.CreateSKUReq{
		ProductId: pr.ProductId, Code: "SMOKE-SKU-" + ts, Name: "冒烟电池", Type: "STANDARD",
	})
	if err != nil {
		fail("建 SKU", err)
	}
	skuId := skr.SkuId
	if _, err := catalog.SetPrice(ctx, &pb.SetPriceReq{
		SkuId: skuId, PriceType: "RETAIL", Amount: "1299.00",
	}); err != nil {
		fail("设价格", err)
	}
	pass(fmt.Sprintf("商品/SKU/价格就绪 sku=%d", skuId))

	// 2. 仓库 + 入库 10 件（biz_no 唯一 → 幂等）
	warehouseId := int64(101489551645622272) // dev seed 仓库
	if _, err := inv.StockIn(ctx, &invpb.StockInReq{
		WarehouseId: warehouseId, SkuId: skuId, Qty: 10,
		BizType: "STOCK_IN", BizNo: biz + "-IN", Remark: "e2esmoke",
	}); err != nil {
		fail("入库", err)
	}
	pass("入库 10 件")

	// 3. 下单（Saga 步骤1；事件异步推进）
	co, err := order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Remark: "e2esmoke",
		Items: []*opb.OrderItemInput{{SkuId: skuId, WarehouseId: warehouseId, Qty: 1, Sn: sn}},
	})
	if err != nil {
		fail("下单", err)
	}
	pass(fmt.Sprintf("下单 %s total=%s", co.OrderNo, co.TotalAmount))

	// 4. 等待步骤2 锁定完成（事件异步 → 轮询状态）
	waitFor(40*time.Second, "锁定库存", func() bool {
		o, err := order.GetOrder(ctx, &opb.GetOrderReq{OrderNo: co.OrderNo})
		return err == nil && (o.Order.Status == "LOCKED" || o.Order.Status == "PAYING" || o.Order.Status == "PAID")
	})

	// 5. 发起支付 + 模拟到账（Saga 步骤3 → order_paid 事件 → 步骤4/5 异步）
	po, err := order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "WECHAT"})
	if err != nil {
		fail("发起支付", err)
	}
	if _, err := fin.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: po.PaymentNo, ChannelTxnId: "MOCK-" + biz,
		PaidAmount: co.TotalAmount, Source: "MOCK",
	}); err != nil {
		fail("模拟到账", err)
	}
	pass("支付确认（dev 模拟网关）" + po.PaymentNo)

	// 6. 等待 Saga 全链完成（含库存 Redis 网关若未预热走 DB 路径）
	waitDeadline := time.Duration(*timeout) * time.Second
	waitFor(waitDeadline, "Saga 推进至 DONE", func() bool {
		o, err := order.GetOrder(ctx, &opb.GetOrderReq{OrderNo: co.OrderNo})
		return err == nil && o.Order.Status == "DONE"
	})
	sg, err := order.GetSaga(ctx, &opb.GetSagaReq{OrderNo: co.OrderNo})
	if err != nil || sg.Saga.Status != "DONE" {
		fail("Saga 终态", fmt.Errorf("status=%v", sg.Saga.Status))
	}
	pass("Saga 全链 DONE（创建→锁定→支付→出库→合同质保）")

	// 7. 发货单 + 轨迹 + 签收（shipment_signed 事件 → 质保起算）
	sh, err := order.CreateShipment(ctx, &opb.CreateShipmentReq{
		OrderNo: co.OrderNo, WarehouseId: warehouseId,
		Items: []*opb.ShipmentItemInput{{SkuId: skuId, Qty: 1, Sn: sn}},
		Carrier: "冒烟物流",
	})
	if err != nil {
		fail("创建发货单", err)
	}
	if _, err := order.AddShipmentTrace(ctx, &opb.AddShipmentTraceReq{
		ShipmentNo: sh.ShipmentNo, Node: "华东分拨", Description: "已发出",
	}); err != nil {
		fail("轨迹录入", err)
	}
	if _, err := order.ConfirmShipmentSigned(ctx, &opb.ConfirmShipmentSignedReq{
		ShipmentNo: sh.ShipmentNo, SignedBy: "冒烟签收人",
	}); err != nil {
		fail("签收确认", err)
	}
	pass("发货→轨迹→签收")

	// 8. 等待质保起算（contract 消费 shipment_signed）
	waitFor(40*time.Second, "质保起算", func() bool {
		w, err := contract.GetWarrantyByTarget(ctx, &ctpb.GetWarrantyByTargetReq{TargetType: "DEVICE", Sn: sn})
		return err == nil && len(w.List) > 0 && w.List[0].Status == "ACTIVE"
	})
	pass("质保起算（warranty_started）sn=" + sn)

	// 9. 出库核验：inventory available 较入库后减少 1（FR-INV-004 唯一写入方）
	gi, err := inv.GetInventory(ctx, &invpb.GetInventoryReq{WarehouseId: warehouseId, SkuId: skuId})
	if err != nil {
		fail("库存核验", err)
	}
	fmt.Printf("  · 终态库存 available=%d locked=%d\n", gi.Inventory.Available, gi.Inventory.Locked)

	fmt.Println("e2esmoke: PASS ✓")
}

func waitFor(d time.Duration, what string, cond func() bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("  ✗ 等待超时: %s（%s）\n", what, d)
	fmt.Println("e2esmoke: FAIL")
	os.Exit(1)
}

