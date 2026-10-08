package main

import (
	"flag"
	"fmt"

	"micro-server/services/party/internal/confcenter"
	"micro-server/services/party/internal/config"
	"micro-server/services/party/internal/confx"
	"micro-server/services/party/internal/server"
	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/party.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（ADR-10：运行时参数热更新；连不上 etcd 用默认值）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterPartyServer(grpcServer, server.NewPartyServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// RPC metadata 桥（02 §9.5）：BFF ctxkit 值 ↔ metadata 还原 + 业务码 gRPC 往返
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
