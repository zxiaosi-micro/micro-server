// paytimeout-test · 阶段验收辅助：建单不支付 → 等待 pay_expire_at 扫描 → PAY_TIMEOUT（ADR-09 无 MQ 延迟消息）。
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"micro-server/services/catalog/pb"
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

	ts := time.Now().Format("0102150405")
	pr, err := catalog.CreateProduct(ctx, &pb.CreateProductReq{Name: "超时测试" + ts, Category: "SMOKE"})
	must(err)
	skr, err := catalog.CreateSKU(ctx, &pb.CreateSKUReq{ProductId: pr.ProductId, Code: "PTO-" + ts, Name: "超时电池", Type: "STANDARD"})
	must(err)
	_, err = catalog.SetPrice(ctx, &pb.SetPriceReq{SkuId: skr.SkuId, PriceType: "RETAIL", Amount: "99.00"})
	must(err)
	_, err = inv.StockIn(ctx, &invpb.StockInReq{WarehouseId: 101489551645622272, SkuId: skr.SkuId, Qty: 3, BizType: "STOCK_IN", BizNo: "PTO-IN-" + ts})
	must(err)
	co, err := order.CreateOrder(ctx, &opb.CreateOrderReq{
		Type: "DEVICE", Items: []*opb.OrderItemInput{{SkuId: skr.SkuId, WarehouseId: 101489551645622272, Qty: 1, Sn: "PTOSN" + ts}},
	})
	must(err)
	fmt.Println("PAYTIMEOUT-ORDER:", co.OrderNo, "expire:", time.UnixMilli(co.PayExpireAt).Format("15:04:05"))
	os.Exit(0)
}

func must(err error) {
	if err != nil {
		fmt.Println("paytimeout fail:", err)
		os.Exit(1)
	}
}
