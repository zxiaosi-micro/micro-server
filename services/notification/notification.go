package main

import (
	"flag"
	"fmt"

	"micro-server/services/notification/internal/confcenter"
	"micro-server/services/notification/internal/config"
	"micro-server/services/notification/internal/confx"
	"micro-server/services/notification/internal/consumer"
	"micro-server/services/notification/internal/server"
	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/notification.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S4-05：频控上限/免打扰兜底热更新，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterNotificationServer(grpcServer, server.NewNotificationServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// notification_request 事件消费（FR-NTF-001 唯一消费方；topic 经 tools/mqinit 预创建）
	consumer.MustStart(c, ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
