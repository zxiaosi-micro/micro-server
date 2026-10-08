package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/finance/internal/confcenter"
	"micro-server/services/finance/internal/config"
	"micro-server/services/finance/internal/confx"
	"micro-server/services/finance/internal/consumer"
	"micro-server/services/finance/internal/server"
	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/finance.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S5-03：mock 网关/渠道降级/渠道开关热调，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterFinanceServer(grpcServer, server.NewFinanceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// 事件消费（order_return_approved → 原路退回；group=finance-refund）
	consumer.MustStart(context.Background(), c, ctx)

	// Outbox Relay（S5 落地：order_paid / payment_refunded 可靠投递）
	startRelay(c, ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// startRelay Outbox Relay：SKIP LOCKED 拉取 event_outbox → Kafka。
func startRelay(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("finance: 未配置 Kafka brokers，Outbox Relay 未启动（事件滞留 outbox 待重投）")
		return
	}
	sender, err := eventbus.NewKqSender(c.Kafka.Brokers, []string{
		eventbus.TopicOrderPaid, eventbus.TopicPaymentRefunded,
	})
	if err != nil {
		logx.Errorf("finance: KqSender 构造失败（Relay 未启动）: %v", err)
		return
	}
	relay := eventbus.NewRelay(sc.Conn, sender, eventbus.RelayConf{
		Interval:  time.Second,
		BatchSize: 100,
		MaxRetry:  16,
	}, eventbus.WithDeadHook(func(ev eventbus.DeadEvent) {
		logx.Errorf("finance: 事件进入死信（告警口径）event_id=%s topic=%s err=%s", ev.EventID, ev.Topic, ev.LastError)
	}))
	go relay.Run(context.WithoutCancel(context.Background()))
	logx.Info("finance: Outbox Relay 已启动")
}
