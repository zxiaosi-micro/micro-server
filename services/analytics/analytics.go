package main

import (
	"context"
	"flag"
	"time"

	"micro-server/services/analytics/internal/confcenter"
	"micro-server/services/analytics/internal/config"
	"micro-server/services/analytics/internal/confx"
	"micro-server/services/analytics/internal/consumer"
	"micro-server/services/analytics/internal/cronx"
	"micro-server/services/analytics/internal/server"
	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/analytics.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S8：导出并发/PDF 行数封顶/对账开关热调，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterAnalyticsServer(grpcServer, server.NewAnalyticsServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// 事件消费（读模型 14 类 + 导出管道；双消费组）
	consumer.MustStart(c, ctx)

	// Outbox Relay（export_requested 自管道 + notification_request 对账站内信）
	startRelay(c, ctx)

	// cron 扫描（ADR-09）：daily_recon（configcenter 开关控制）
	registerCron(ctx)

	logx.Infof("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// startRelay Outbox Relay：SKIP LOCKED 拉取 event_outbox → Kafka（PushWithKey 保序）。
func startRelay(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("analytics: 未配置 Kafka brokers，Outbox Relay 未启动（事件滞留 outbox 待重投）")
		return
	}
	sender, err := eventbus.NewKqSender(c.Kafka.Brokers, []string{eventbus.TopicExportRequested, eventbus.TopicNotificationRequest})
	if err != nil {
		logx.Errorf("analytics: KqSender 构造失败（Relay 未启动）: %v", err)
		return
	}
	relay := eventbus.NewRelay(sc.Conn, sender, eventbus.RelayConf{
		Interval:  time.Second,
		BatchSize: 100,
		MaxRetry:  16,
	}, eventbus.WithDeadHook(func(ev eventbus.DeadEvent) {
		logx.Errorf("analytics: 事件进入死信 event_id=%s topic=%s err=%s", ev.EventID, ev.Topic, ev.LastError)
	}))
	go relay.Run(context.WithoutCancel(context.Background()))
	logx.Info("analytics: Outbox Relay 已启动")
}

// registerCron ADR-09 扫描器注册（登记 + last_run 对账）。
func registerCron(sc *svc.ServiceContext) {
	cronx.RegisterAll(sc)

	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
}
