// Package forwarder · EMQX 共享订阅 → Kafka 缓冲（S6-03，ADR-04）。
//
// 背压点（FR-IOT-004）：handler 内同步 produce iot_telemetry_raw，成功才返回（paho QoS1
// handler 返回后才发 PUBACK）——"MQ 成功才 ack MQTT"；produce 失败返回 error 由 paho 内部
// 依 QoS1 重投（broker 未收到 PUBACK 会重发，at-least-once，写入侧同 ts 覆盖幂等兜底）。
package forwarder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// Pusher Kafka 生产抽象（kq.Pusher；测试可 mock）。
type Pusher interface {
	PushWithKey(key, val string) error
}

// Forwarder 共享订阅转发器。
type Forwarder struct {
	pusher Pusher
}

// New 构造。
func New(pusher Pusher) *Forwarder {
	return &Forwarder{pusher: pusher}
}

// Run 建立 MQTT 连接并订阅（阻塞至 ctx 取消；断线自动重连）。
func (f *Forwarder) Run(ctx context.Context, broker, user, pass, clientID, topicFilter string) error {
	handler := func(_ paho.Client, m paho.Message) {
		// topic 形状 up/{tenant}/{pk}/{sn}/telemetry（ACL 最小主题，02 §7.2）
		tenant, pk, sn, err := parseTopic(m.Topic())
		if err != nil {
			logx.Errorf("forwarder: 非法主题丢弃 topic=%s err=%v", m.Topic(), err)
			return // 畸形消息 ack 丢弃（不无限重投）
		}
		msg := TelemetryMsg{
			TenantId:   tenant,
			ProductKey: pk,
			Sn:         sn,
			Payload:    string(m.Payload()),
			Ts:         time.Now().UnixMilli(),
		}
		val, _ := json.Marshal(msg)
		if err := f.pusher.PushWithKey(sn, string(val)); err != nil {
			logx.Errorf("forwarder: Kafka produce 失败（MQTT 不 ack，QoS1 重投）sn=%s err=%v", sn, err)
			panic(err) // panic 阻止 handler 正常返回 → 无 PUBACK → broker 重发
		}
	}

	for {
		opts := paho.NewClientOptions().
			AddBroker(broker).
			SetClientID(clientID).
			SetUsername(user).
			SetPassword(pass).
			SetAutoReconnect(true).
			SetConnectRetry(true).
			SetConnectRetryInterval(5 * time.Second).
			SetCleanSession(false). // 持久会话：离线期间 QoS1 消息由 broker 保留
			SetDefaultPublishHandler(handler)
		cli := paho.NewClient(opts)
		if token := cli.Connect(); token.Wait() && token.Error() != nil {
			logx.Errorf("forwarder: MQTT 连接失败（5s 重试）: %v", token.Error())
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}
		if token := cli.Subscribe(topicFilter, 1, nil); token.Wait() && token.Error() != nil {
			logx.Errorf("forwarder: 订阅失败（5s 重连）filter=%s err=%v", topicFilter, token.Error())
			cli.Disconnect(250)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				continue
			}
		}
		logx.Infof("forwarder: 已订阅 %s（共享组 ingest）", topicFilter)
		<-ctx.Done()
		cli.Disconnect(250)
		return ctx.Err()
	}
}

// TelemetryMsg iot_telemetry_raw 消息体（forwarder→writer 契约）。
type TelemetryMsg struct {
	TenantId   int64  `json:"tenant_id"`
	ProductKey string `json:"product_key"`
	Sn         string `json:"sn"`
	Payload    string `json:"payload"` // 设备原始报文（JSON 五指标或厂商格式）
	Ts         int64  `json:"ts"`      // 平台接收时刻毫秒
}

// parseTopic 解析 up/{tenant}/{pk}/{sn}/telemetry。
func parseTopic(topic string) (tenant int64, pk, sn string, err error) {
	parts := splitPath(topic)
	if len(parts) != 5 || parts[0] != "up" || parts[4] != "telemetry" {
		return 0, "", "", fmt.Errorf("topic 形状不符: %s", topic)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &tenant); err != nil {
		return 0, "", "", fmt.Errorf("tenant 非法: %s", parts[1])
	}
	return tenant, parts[2], parts[3], nil
}

func splitPath(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '/' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

// KqPusher kq.Pusher 适配（PushWithKey 同步发送，eventbus 封装）。
type KqPusher struct{ p *eventbus.KqPusher }

// NewKqPusher 构造。
func NewKqPusher(brokers []string, topic string) (*KqPusher, error) {
	p, err := eventbus.NewKqPusher(brokers, topic)
	if err != nil {
		return nil, err
	}
	return &KqPusher{p: p}, nil
}

// PushWithKey 保序发送（key=sn，per-aggregate 有序）。
func (k *KqPusher) PushWithKey(key, val string) error {
	return k.p.PushWithKey(context.Background(), key, val)
}

// Close Flush 优雅关闭。
func (k *KqPusher) Close() error { return k.p.Close() }
