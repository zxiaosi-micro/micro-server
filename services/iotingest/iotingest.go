package main

import (
	"context"
	"flag"

	"micro-server/services/iotingest/internal/confcenter"
	"micro-server/services/iotingest/internal/config"
	"micro-server/services/iotingest/internal/confx"
	"micro-server/services/iotingest/internal/forwarder"
	"micro-server/services/iotingest/internal/ocpp"
	"micro-server/services/iotingest/internal/rules"
	"micro-server/services/iotingest/internal/tdengine"
	"micro-server/services/iotingest/internal/writer"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

var configFile = flag.String("f", "etc/iotingest.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// 规则配置订阅（/micro/config/iotingest/rules；30s TTL + 变更失效，FR-IOT-009）
	confcenter.MustListen(c.EtcdHosts(), c.ConfigKey+"/rules")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	group := service.ServiceGroup{}

	// ---- TDengine 客户端 + 建库（幂等，启动期 fail-fast）----
	td := tdengine.New(c.Tdengine.RestUrl, c.Tdengine.User, c.Tdengine.Pass, c.Tdengine.Db)
	if err := td.EnsureDatabase(ctx); err != nil {
		logx.Errorf("iotingest: TDengine 建库失败（继续启动，写入失败会回灌）: %v", err)
	}

	// ---- writer：Kafka 消费 → BulkExecutor(500/200ms) → TDengine + 影子 ----
	var w *writer.Writer
	var shadowStore writer.ShadowStore
	if c.ShadowRedis.Host != "" {
		if rd, err := redis.NewRedis(c.ShadowRedis); err == nil {
			shadowStore = writer.NewShadowStore(rd)
		}
	}
	if len(c.Kafka.Brokers) > 0 {
		var rep writer.Replayer
		if pusher, err := forwarder.NewKqPusher(c.Kafka.Brokers, c.Kafka.Topic); err == nil {
			rep = &kqReplayer{p: pusher}
			defer func() { _ = pusher.Close() }()
		} else {
			logx.Errorf("iotingest: 回灌 Pusher 构造失败: %v", err)
		}
		w = writer.New(td, shadowStore, rep)

		// 写入失败回灌 = 重新消费 iot_telemetry_raw（Offset first + 幂等 ts 覆盖兜底）
		q := kq.MustNewQueue(kq.KqConf{
			Brokers: c.Kafka.Brokers, Group: c.Kafka.Group, Topic: c.Kafka.Topic,
			Offset: "first", Conns: 1, Consumers: 1, Processors: 1,
		}, w)
		group.Add(startStopFunc{start: func() { go q.Start() }, stop: func() { q.Stop() }})
		logx.Infof("iotingest: writer 消费已启动 topic=%s group=%s", c.Kafka.Topic, c.Kafka.Group)
	}

	// ---- rules：越限初筛 → alert_candidate ----
	var alertPusher *forwarder.KqPusher
	if len(c.Kafka.Brokers) > 0 {
		if ap, err := forwarder.NewKqPusher(c.Kafka.Brokers, c.AlertTopic); err == nil {
			alertPusher = ap
			defer func() { _ = ap.Close() }()
		}
	}
	ev := rules.NewEvaluator(c.Rules.DefaultRules, c.Rules.TtlSec, alertPusher)
	confcenter.OnReload(ev.Reload)

	// ---- forwarder：EMQX 共享订阅 → iot_telemetry_raw（MQ 成功才 ack MQTT）----
	// 与 ocpp 共用"遥测 sink"抽象：Kafka push + 规则初筛。
	telemetrySink := &telemetrySink{pusher: kqPusherOf(c), ev: ev}
	if c.Mqtt.Broker != "" && len(c.Kafka.Brokers) > 0 {
		pusher := mustPusher(c)
		fw := forwarder.New(pusher)
		group.Add(startStopFunc{
			start: func() {
				go func() {
					_ = fw.Run(ctx, c.Mqtt.Broker, c.Mqtt.PlatformUser, c.Mqtt.PlatformPass, c.Mqtt.ClientId, c.Mqtt.TopicFilter)
				}()
			},
			stop: func() { _ = pusher.Close() },
		})
	} else {
		logx.Info("iotingest: MQTT/Kafka 未完整配置，forwarder 未启动")
	}

	// ---- ocpp：CSMS WebSocket :8182 ----
	ocppSrv := ocpp.NewServer("0.0.0.0:8182", telemetrySink, nil)
	group.Add(startStopFunc{
		start: func() { go func() { _ = ocppSrv.Run(ctx) }() },
		stop:  func() {},
	})

	// 优雅退出（drain 批量缓冲后关 Kafka Flush，02 §9.6）
	proc.AddShutdownListener(func() {
		cancel()
		if w != nil {
			w.Flush()
		}
	})
	group.Start()
}

// ---- 接线小件 ----

type startStopFunc struct {
	start func()
	stop  func()
}

func (s startStopFunc) Start() { s.start() }
func (s startStopFunc) Stop()  { s.stop() }

func mustPusher(c config.Config) *forwarder.KqPusher {
	p, err := forwarder.NewKqPusher(c.Kafka.Brokers, c.Kafka.Topic)
	if err != nil {
		logx.Must(err)
	}
	return p
}

func kqPusherOf(c config.Config) *forwarder.KqPusher {
	if len(c.Kafka.Brokers) == 0 {
		return nil
	}
	p, err := forwarder.NewKqPusher(c.Kafka.Brokers, c.Kafka.Topic)
	if err != nil {
		return nil
	}
	return p
}

// kqReplayer Kafka 回灌适配（写失败重投 iot_telemetry_raw，02 §9.6 组件语义）。
type kqReplayer struct{ p *forwarder.KqPusher }

func (k *kqReplayer) Replay(key, val string) error {
	return k.p.PushWithKey(key, val)
}

// telemetrySink 统一遥测出口：Kafka push（forwarder 语义）+ 越限初筛（rules）。
// ocpp 与 forwarder 共用；写遥测的同一载荷做候选判定（origin source 区分）。
type telemetrySink struct {
	pusher *forwarder.KqPusher
	ev     *rules.Evaluator
}

func (t *telemetrySink) Push(tenantId int64, pk, sn, payload string) error {
	if t.pusher != nil {
		if err := t.pusher.PushWithKey(sn, payload); err != nil {
			return err
		}
	}
	if t.ev != nil {
		var m struct {
			Ts int64 `json:"ts"`
		}
		_ = jsonUnmarshal(payload, &m)
		t.ev.Evaluate(context.Background(), tenantId, pk, sn, extractInnerPayload(payload), m.Ts, "ocpp")
	}
	return nil
}
