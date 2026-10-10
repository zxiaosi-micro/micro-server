// 事件消费启动（device-asset 消费组，全局唯一，E2）：stock_in / stock_out / cmd_ack。
// kq.Start 阻塞（09:40 事故）：一律 go q.Start() 后台化；去重/重投/死信由 eventbus.Subscriber 承担。

package consumer

import (
	"context"

	"micro-server/services/device/internal/config"
	"micro-server/services/device/internal/logic"
	"micro-server/services/device/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// MustStart 消费三 topic（同一消费组；未配置 brokers 时仅 RPC 面——本地开发免 MQ 启动）。
func MustStart(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("device: 未配置 Kafka brokers，事件消费未启动（仅 RPC 面）")
		return
	}
	topics := []struct {
		topic   string
		handler eventbus.Handler
	}{
		{eventbus.TopicStockIn, logic.HandleStockIn(sc)},
		{eventbus.TopicStockOut, logic.HandleStockOut(sc)},
		{eventbus.TopicCmdAck, logic.HandleCmdAck(sc)},
	}
	for _, t := range topics {
		sub, err := eventbus.NewSubscriber(sc.Conn, t.handler, eventbus.SubConf{Group: c.Kafka.Group})
		if err != nil {
			logx.Must(err)
		}
		q := kq.MustNewQueue(kq.KqConf{
			Brokers:    c.Kafka.Brokers,
			Group:      c.Kafka.Group,
			Topic:      t.topic,
			Offset:     "first", // 新组从头消费；生命周期日志唯一键 + CAS 守卫兜底重放（at-least-once 口径）
			Conns:      1,
			Consumers:  1,
			Processors: 1,
		}, kqHandler{sub})
		// kq.Start 阻塞——必须后台化（09:40 事故：同步调用吞掉 s.Start()）
		go q.Start()
		logx.Infof("device: %s 消费已启动 group=%s", t.topic, c.Kafka.Group)
	}
}

// kqHandler 签名适配：kq.ConsumeHandler（带 ctx）→ eventbus.Subscriber（key,val）。
type kqHandler struct {
	sub *eventbus.Subscriber
}

func (h kqHandler) Consume(_ context.Context, key, val string) error {
	return h.sub.Consume(key, val)
}
