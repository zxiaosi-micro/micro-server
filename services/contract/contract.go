package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/contract/internal/confcenter"
	"micro-server/services/contract/internal/config"
	"micro-server/services/contract/internal/confx"
	"micro-server/services/contract/internal/consumer"
	"micro-server/services/contract/internal/cronx"
	"micro-server/services/contract/internal/logic"
	"micro-server/services/contract/internal/server"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/contract.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S5-04：打印稿开关/默认质保月数热调，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterContractServer(grpcServer, server.NewContractServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// 事件消费（shipment_signed / device_activated → 质保起算；group=contract-warranty）
	consumer.MustStart(context.Background(), c, ctx)

	// Outbox Relay（warranty_started 可靠投递）
	startRelay(c, ctx)

	// cron：质保到期扫描（E16：登记 + last_run 指标）
	registerCron(ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// startRelay Outbox Relay：SKIP LOCKED 拉取 event_outbox → Kafka。
func startRelay(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("contract: 未配置 Kafka brokers，Outbox Relay 未启动（事件滞留 outbox 待重投）")
		return
	}
	sender, err := eventbus.NewKqSender(c.Kafka.Brokers, []string{eventbus.TopicWarrantyStarted})
	if err != nil {
		logx.Errorf("contract: KqSender 构造失败（Relay 未启动）: %v", err)
		return
	}
	relay := eventbus.NewRelay(sc.Conn, sender, eventbus.RelayConf{
		Interval:  time.Second,
		BatchSize: 100,
		MaxRetry:  16,
	}, eventbus.WithDeadHook(func(ev eventbus.DeadEvent) {
		logx.Errorf("contract: 事件进入死信（告警口径）event_id=%s topic=%s err=%s", ev.EventID, ev.Topic, ev.LastError)
	}))
	go relay.Run(context.WithoutCancel(context.Background()))
	logx.Info("contract: Outbox Relay 已启动")
}

// registerCron 质保到期扫描（ACTIVE 且 end_at 到期 → EXPIRED）。
func registerCron(sc *svc.ServiceContext) {
	cronx.Register(cronx.Task{
		Name:     "contract-warranty-expiry-scan",
		Interval: time.Minute,
		Run: func(ctx context.Context) error {
			return logic.ScanWarrantyExpiry(ctx, sc)
		},
	})
	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
}
