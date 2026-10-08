package svc

import (
	"context"
	"fmt"

	"micro-server/services/order/internal/config"
	"micro-server/services/order/internal/model"

	catclient "micro-server/services/catalog/catalog"
	ctclient "micro-server/services/contract/contract"
	finclient "micro-server/services/finance/finance"
	invclient "micro-server/services/inventory/inventory"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
)

// ServiceContext order 服务根装配。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器（订单号/退货号/发运号 = 前缀 + 雪花）。
	Snowflake  *snowflake.Node
	snowCancel func()

	// 下游 RPC 客户端（Saga 步骤动作；未配置为 nil，步骤报可重试错误——冒烟/降级口径见 02 §10）。
	Inventory invclient.Inventory
	Finance   finclient.Finance
	Contract  ctclient.Contract
	Catalog   catclient.Catalog
}

// Models model 聚合。
type Models struct {
	Order         model.OrderModel
	OrderItem     model.OrderItemModel
	Saga          model.SagaModel
	Shipment      model.ShipmentModel
	ShipmentTrace model.ShipmentTraceModel
	ShipmentItem  model.ShipmentItemModel
	ReturnOrder   model.ReturnOrderModel
	ReturnItem    model.ReturnItemModel
}

// dial 构造下游客户端（metadata 桥：uid/tenant 随链路透传，02 §9.5）。
func dial[T any](c zrpc.RpcClientConf, newClient func(zrpc.Client) T) T {
	cli := zrpc.MustNewClient(c, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
	return newClient(cli)
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	cacheConf := c.Cache
	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Snowflake:  node,
		snowCancel: snowCancel,
		Models: &Models{
			Order:         model.NewOrderModel(conn, cacheConf),
			OrderItem:     model.NewOrderItemModel(conn, cacheConf),
			Saga:          model.NewSagaModel(conn, cacheConf),
			Shipment:      model.NewShipmentModel(conn, cacheConf),
			ShipmentTrace: model.NewShipmentTraceModel(conn, cacheConf),
			ShipmentItem:  model.NewShipmentItemModel(conn, cacheConf),
			ReturnOrder:   model.NewReturnOrderModel(conn, cacheConf),
			ReturnItem:    model.NewReturnItemModel(conn, cacheConf),
		},
	}

	// 下游 RPC（config 缺省则 nil——Saga 对应步骤会报可重试错误并最终进人工队列）
	if len(c.InventoryRpc.Endpoints) > 0 || c.InventoryRpc.Target != "" {
		sc.Inventory = dial(c.InventoryRpc, invclient.NewInventory)
	}
	if len(c.FinanceRpc.Endpoints) > 0 || c.FinanceRpc.Target != "" {
		sc.Finance = dial(c.FinanceRpc, finclient.NewFinance)
	}
	if len(c.ContractRpc.Endpoints) > 0 || c.ContractRpc.Target != "" {
		sc.Contract = dial(c.ContractRpc, ctclient.NewContract)
	}
	if len(c.CatalogRpc.Endpoints) > 0 || c.CatalogRpc.Target != "" {
		sc.Catalog = dial(c.CatalogRpc, catclient.NewCatalog)
	}

	logx.Infof("order 就绪：db=micro_order 下游 inventory=%v finance=%v contract=%v catalog=%v",
		sc.Inventory != nil, sc.Finance != nil, sc.Contract != nil, sc.Catalog != nil)
	return sc
}
