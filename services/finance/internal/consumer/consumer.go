// 事件消费启动（finance-refund 消费组，全局唯一，E2）：order_return_approved → 原路退回。

package consumer

import (
	"context"
	"time"

	"micro-server/services/finance/internal/config"
	"micro-server/services/finance/internal/logic"
	"micro-server/services/finance/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// MustStart 消费退货审批事件（未配置 brokers 时仅 RPC 面）。
func MustStart(ctx context.Context, c config.Config, sc *svc.ServiceContext) {
	_ = ctx
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("finance: 未配置 Kafka brokers，事件消费未启动（仅 RPC 面）")
		return
	}
	sub, err := eventbus.NewSubscriber(sc.Conn, logic.HandleReturnApproved(sc), eventbus.SubConf{Group: c.Kafka.Group})
	if err != nil {
		logx.Must(err)
	}
	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.Group,
		Topic:      eventbus.TopicOrderReturnApproved,
		Offset:     "first", // 新组从头消费；event_dedup 同事务去重兜底重放（at-least-once 口径）
		Conns:      1,
		Consumers:  1,
		Processors: 1,
		MinBytes:   1, // 显式传：KqConf 字面量不走 conf 解码，default 标签不生效（kq 空转 known-pit）；1=有数据即返保低延迟
		MaxBytes:   10485760,
	}, kqHandler{sub}, kq.WithMaxWait(10*time.Second)) // 默认 1s 撞 kafka-go RTT 算术边界 → maxWait=0 空转（v4/04）
	// kq.Start 阻塞（09:40 事故）：后台化
	go q.Start()
	logx.Infof("finance: %s 消费已启动 group=%s", eventbus.TopicOrderReturnApproved, c.Kafka.Group)
}

// kqHandler 签名适配：kq.ConsumeHandler（带 ctx）→ eventbus.Subscriber（key,val）。
type kqHandler struct {
	sub *eventbus.Subscriber
}

func (h kqHandler) Consume(_ context.Context, key, val string) error {
	return h.sub.Consume(key, val)
}
