// Package consumer · ops 事件消费（alert_candidate/cmd_failed/ota_paused/device_anomaly/station_created）。
// kq 单队列单 topic：每 topic 独立建队（同消费组），handler 绑定 topic 分流。
package consumer

import (
	"context"
	"encoding/json"
	"time"

	"micro-server/services/ops/internal/config"
	"micro-server/services/ops/internal/logic"
	"micro-server/services/ops/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// topics ops 订阅清单（E2 先建后用；group 全局唯一 ops-consumer）。
var topics = []string{
	eventbus.TopicAlertCandidate,
	eventbus.TopicCmdFailed,
	eventbus.TopicOtaPaused,
	eventbus.TopicDeviceAnomaly,
	eventbus.TopicStationCreated,
}

// MustStart 启动事件消费（未配置 brokers 时静默跳过；失败 DB 退避重投由 kq 承担）。
func MustStart(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("ops: 未配置 Kafka brokers，事件消费未启动")
		return
	}
	group := c.Kafka.Group
	if group == "" {
		group = "ops-consumer"
	}
	for _, topic := range topics {
		q := kq.MustNewQueue(kq.KqConf{
			Brokers:    c.Kafka.Brokers,
			Group:      group,
			Topic:      topic,
			Offset:     "first", // 新消费组配 first + event_dedup 去重（known-pit：last 会漏消息）
			Conns:      1,
			Consumers:  1,
			Processors: 1,
			MinBytes:   1, // 显式传：KqConf 字面量不走 conf 解码，default 标签不生效（kq 空转 known-pit）；1=有数据即返保低延迟
			MaxBytes:   10485760,
		}, &handler{sc: sc, topic: topic}, kq.WithMaxWait(10*time.Second)) // 默认 1s 撞 kafka-go RTT 算术边界 → maxWait=0 空转（v4/04）
		// q.Start() 是阻塞调用：必须 go 化（09:40 事故：同步调用吞掉 s.Start()）
		go q.Start()
	}
	logx.Infof("ops: 事件消费已启动 group=%s topics=%d 个", group, len(topics))
}

// handler kq ConsumeHandler（topic 绑定式）。
type handler struct {
	sc    *svc.ServiceContext
	topic string
}

// Consume 解包分流：alert_candidate 为裸 payload（iotingest 直推，无信封），
// 其余为 eventbus.Envelope（信封解析 + dedup + 租户恢复）。
func (h *handler) Consume(_ context.Context, _ string, val string) error {
	if h.topic == eventbus.TopicAlertCandidate {
		return logic.ConsumeCandidateRaw(context.Background(), h.sc, []byte(val))
	}
	var env eventbus.Envelope
	if err := json.Unmarshal([]byte(val), &env); err != nil {
		logx.Errorf("ops: 信封解析失败 topic=%s（丢弃毒消息）: %v", h.topic, err)
		return nil
	}
	if err := logic.ConsumeEvent(context.Background(), h.sc, h.topic, env); err != nil {
		logx.Errorf("ops: 事件消费失败 event=%s topic=%s: %v", env.EventID, h.topic, err)
		return err // 交还 kq 退避重投
	}
	return nil
}
