// docktest · 阶段验收辅助：停 contract 后建单支付，验证 Saga 停靠步骤5；恢复 contract 后自动续推。
package main

import (
	"context"
	"fmt"
	"os"
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

func main() {
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), 9000000000000000001), 9000000000000000002)
	dial := func(port int) grpc.ClientConnInterface {
		conn := zrpc.MustNewClient(zrpc.RpcClientConf{
			Endpoints: []string{fmt.Sprintf("127.0.0.1:%d", port)},
			NonBlock:  true, Timeout: 8000,
		}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
		return conn.Conn()
	}
	catalog := pb.NewCatalogClient(dial(8083))
	inv := invpb.NewInventoryClient(dial(8084))
	order := opb.NewOrderClient(dial(8090))
	fin := finpb.NewFinanceClient(dial(8092))

	ts := time.Now().Format("0102150405")
	pr, err := catalog.CreateProduct(ctx, &pb.CreateProductReq{Name: "停靠测试" + ts, Category: "SMOKE"})
	must(err)
	skr, err := catalog.CreateSKU(ctx, &pb.CreateSKUReq{ProductId: pr.ProductId, Code: "DOCK-" + ts, Name: "停靠电池", Type: "STANDARD"})
	must(err)
	_, err = catalog.SetPrice(ctx, &pb.SetPriceReq{SkuId: skr.SkuId, PriceType: "RETAIL", Amount: "199.00"})
	must(err)
	_, err = inv.StockIn(ctx, &invpb.StockInReq{WarehouseId: 101489551645622272, SkuId: skr.SkuId, Qty: 5, BizType: "STOCK_IN", BizNo: "DOCK-IN-" + ts})
	must(err)
	co, err := order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Items: []*opb.OrderItemInput{{SkuId: skr.SkuId, WarehouseId: 101489551645622272, Qty: 1, Sn: "DOCKSN" + ts}},
	})
	must(err)
	waitFor(40*time.Second, "锁定", func() bool {
		o, e := order.GetOrder(ctx, &opb.GetOrderReq{OrderNo: co.OrderNo})
		return e == nil && o.Order.Status == "LOCKED"
	})
	po, err := order.PayOrder(ctx, &opb.PayOrderReq{OrderNo: co.OrderNo, Channel: "WECHAT"})
	must(err)
	_, err = fin.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{PaymentNo: po.PaymentNo, ChannelTxnId: "DOCK-TXN-" + ts, PaidAmount: "199.00", Source: "MOCK"})
	must(err)
	fmt.Println("DOCKTEST-ORDER:", co.OrderNo)
	os.Exit(0)
}

func must(err error) {
	if err != nil {
		fmt.Println("docktest fail:", err)
		os.Exit(1)
	}
}

func waitFor(d time.Duration, what string, cond func() bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
	fmt.Println("docktest timeout:", what)
	os.Exit(1)
}
