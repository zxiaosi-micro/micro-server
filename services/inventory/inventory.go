package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/inventory/internal/confcenter"
	"micro-server/services/inventory/internal/config"
	"micro-server/services/inventory/internal/confx"
	"micro-server/services/inventory/internal/cronx"
	"micro-server/services/inventory/internal/logic"
	"micro-server/services/inventory/internal/server"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/inventory.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（ADR-10：运行时参数热更新；连不上 etcd 用默认值）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterInventoryServer(grpcServer, server.NewInventoryServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// RPC metadata 桥（02 §9.5）：BFF ctxkit 值 ↔ metadata 还原 + 业务码 gRPC 往返
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	registerCron(ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// registerCron 定时任务登记（ADR-09：内存态定时只做加速，DB 是兜底事实源）。
// Redis 预扣计数器与 DB available 可能漂移（回滚 INCRBY 失败等），由对账任务周期收敛。
func registerCron(sc *svc.ServiceContext) {
	cronx.Register(cronx.Task{
		Name:     "inventory-redis-reconcile",
		Interval: 30 * time.Second,
		Run: func(ctx context.Context) error {
			return logic.ReconcileRedis(ctx, sc)
		},
	})
	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
}
