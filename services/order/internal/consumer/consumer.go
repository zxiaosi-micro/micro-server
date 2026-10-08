// 事件消费启动（order-saga 消费组，全局唯一，E2）：order_paid / stock_out / payment_refunded。
// kq.Start 阻塞（09:40 事故）：一律 go q.Start() 后台化；去重/重投/死信由 eventbus.Subscriber 承担。

package consumer

import (
	"context"

	"micro-server/services/order/internal/config"
	"micro-server/services/order/internal/logic"
	"micro-server/services/order/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// MustStart 消费三 topic（同一消费组；未配置 brokers 时仅 RPC 面——本地开发免 MQ 启动）。
func MustStart(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("order: 未配置 Kafka brokers，事件消费未启动（仅 RPC 面）")
		return
	}
	topics := []struct {
		topic   string
		handler eventbus.Handler
	}{
		{eventbus.TopicOrderPaid, logic.HandleOrderPaid(sc)},
		{eventbus.TopicStockOut, logic.HandleStockOut(sc)},
		{eventbus.TopicPaymentRefunded, logic.HandlePaymentRefunded(sc)},
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
			Offset:     "first",  // 新组从头消费；event_dedup 同事务去重兜底重放（at-least-once 口径）
			Conns:      1,
			Consumers:  1,
			Processors: 1,
		}, kqHandler{sub})
		// kq 队列托管：Start 阻塞消费（内部自旋重连），挂后台 goroutine——
		// 服务主面是 RPC，不得被 MQ 消费阻塞启动（09:40 事故：q.Start 同步调用吞掉 s.Start()）。
		go q.Start()
		logx.Infof("order: %s 消费已启动 group=%s", t.topic, c.Kafka.Group)
	}
}

// kqHandler 签名适配：kq.ConsumeHandler（带 ctx）→ eventbus.Subscriber（key,val）。
// ctx 仅用于 kq 内部取消传播；Subscriber 内部自建含租户/trace 的消费上下文。
type kqHandler struct {
	sub *eventbus.Subscriber
}

func (h kqHandler) Consume(_ context.Context, key, val string) error {
	return h.sub.Consume(key, val)
}
