package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/device/internal/confcenter"
	"micro-server/services/device/internal/config"
	"micro-server/services/device/internal/confx"
	"micro-server/services/device/internal/consumer"
	"micro-server/services/device/internal/cronx"
	"micro-server/services/device/internal/logic"
	"micro-server/services/device/internal/server"
	"micro-server/services/device/internal/svc"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/device.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S6-01：ACK 超时/重试退避/OTA 批量/凭证开关热调，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterDeviceServer(grpcServer, server.NewDeviceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// 事件消费（stock_in/stock_out/cmd_ack；group=device-asset）
	consumer.MustStart(c, ctx)

	// Outbox Relay（device_activated/cmd_failed/ota_paused 可靠投递）
	startRelay(c, ctx)

	// cron 推进器（ADR-09；登记 + last_run 指标，E16）：
	//   ①指令重试扫描（TimingWheel 失联/重启后持久兜底；超限 FAILED→告警）
	//   ②OTA 分批下发扫描（灰度推进 + 失败率超阈自动 PAUSED + 完成判定）
	registerCron(c, ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// startRelay Outbox Relay：SKIP LOCKED 拉取 event_outbox → Kafka（PushWithKey 保序）。
func startRelay(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("device: 未配置 Kafka brokers，Outbox Relay 未启动（事件滞留 outbox 待重投）")
		return
	}
	sender, err := eventbus.NewKqSender(c.Kafka.Brokers, []string{
		eventbus.TopicDeviceActivated, eventbus.TopicCmdFailed, eventbus.TopicOtaPaused,
	})
	if err != nil {
		logx.Errorf("device: KqSender 构造失败（Relay 未启动）: %v", err)
		return
	}
	relay := eventbus.NewRelay(sc.Conn, sender, eventbus.RelayConf{
		Interval:  time.Second,
		BatchSize: 100,
		MaxRetry:  16,
	}, eventbus.WithDeadHook(func(ev eventbus.DeadEvent) {
		logx.Errorf("device: 事件进入死信（告警口径）event_id=%s topic=%s err=%s", ev.EventID, ev.Topic, ev.LastError)
	}))
	go relay.Run(context.WithoutCancel(context.Background()))
	logx.Info("device: Outbox Relay 已启动")
}

// registerCron ADR-09 扫描器注册（登记 + last_run 对账）。
func registerCron(c config.Config, sc *svc.ServiceContext) {
	cronx.Register(cronx.Task{
		Name:     "device-cmd-retry-scan",
		Interval: 30 * time.Second,
		Run: func(ctx context.Context) error {
			return logic.ScanCmdRetry(ctx, sc)
		},
	})
	cronx.Register(cronx.Task{
		Name: "device-ota-dispatch-scan",
		Interval: func() time.Duration {
			if c.OtaDispatchIntervalSec > 0 {
				return time.Duration(c.OtaDispatchIntervalSec) * time.Second
			}
			return 30 * time.Second
		}(),
		Run: func(ctx context.Context) error {
			return logic.ScanOtaDispatch(ctx, sc)
		},
	})
	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
}
