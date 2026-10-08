package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/order/internal/confcenter"
	"micro-server/services/order/internal/config"
	"micro-server/services/order/internal/confx"
	"micro-server/services/order/internal/consumer"
	"micro-server/services/order/internal/cronx"
	"micro-server/services/order/internal/logic"
	"micro-server/services/order/internal/server"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/order.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// configcenter listener（S5-02：支付超时分钟/扫描批量热调，ADR-10）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterOrderServer(grpcServer, server.NewOrderServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(authz.ServerInterceptor)
	defer s.Stop()

	// 事件消费（order_paid/stock_out/payment_refunded；group=order-saga）
	consumer.MustStart(c, ctx)

	// Outbox Relay（S5 落地：order_created/cancelled/pay_timeout/return_approved/shipment_signed 可靠投递）
	startRelay(c, ctx)

	// cron 推进器（ADR-09；登记 + last_run 指标，E16）：
	//   ①支付超时扫描（pay_expire_at 到期 → 取消+释放库存+order_pay_timeout）
	//   ②Saga 重试扫描（next_retry_at 退避 1m/5m/30m；超限 → MANUAL 人工队列）
	//   ③人工队列积压日志（saga_stuck_orders 观测口径）
	registerCron(c, ctx)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}

// startRelay Outbox Relay：SKIP LOCKED 拉取 event_outbox → Kafka（PushWithKey 保序）。
func startRelay(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("order: 未配置 Kafka brokers，Outbox Relay 未启动（事件滞留 outbox 待重投）")
		return
	}
	sender, err := eventbus.NewKqSender(c.Kafka.Brokers, []string{
		eventbus.TopicOrderCreated, eventbus.TopicOrderCancelled, eventbus.TopicOrderPayTimeout,
		eventbus.TopicOrderReturnApproved, eventbus.TopicShipmentSigned,
	})
	if err != nil {
		logx.Errorf("order: KqSender 构造失败（Relay 未启动）: %v", err)
		return
	}
	relay := eventbus.NewRelay(sc.Conn, sender, eventbus.RelayConf{
		Interval:  time.Second,
		BatchSize: 100,
		MaxRetry:  16,
	}, eventbus.WithDeadHook(func(ev eventbus.DeadEvent) {
		logx.Errorf("order: 事件进入死信（告警口径）event_id=%s topic=%s err=%s", ev.EventID, ev.Topic, ev.LastError)
	}))
	go relay.Run(context.WithoutCancel(context.Background()))
	logx.Info("order: Outbox Relay 已启动")
}

// registerCron ADR-09 推进器注册（30s 扫描周期；last_run 经 cronx.LastRun 暴露）。
func registerCron(c config.Config, sc *svc.ServiceContext) {
	cronx.Register(cronx.Task{
		Name:     "order-pay-timeout-scan",
		Interval: 30 * time.Second,
		Run: func(ctx context.Context) error {
			return logic.ScanPayTimeout(ctx, sc)
		},
	})
	cronx.Register(cronx.Task{
		Name:     "order-saga-retry-scan",
		Interval: 30 * time.Second,
		Run: func(ctx context.Context) error {
			return logic.ScanSagaRetry(ctx, sc)
		},
	})
	cronx.Register(cronx.Task{
		Name:     "order-saga-manual-queue-watch",
		Interval: time.Minute,
		Run: func(ctx context.Context) error {
			if n := logic.CountManualSaga(ctx, sc); n > 0 {
				logx.Errorf("saga 人工队列积压 saga_stuck_orders=%d（需人工介入，前端订单详情可重推）", n)
			}
			return nil
		},
	})
	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
	_ = redis.New // 保持 redis 引用位（行缓存经 cache.CacheConf 注入）
}
