// Package consumer · notification_request 事件消费（FR-NTF-001：本服务是唯一消费方）。
//
// 链路：kq 订阅 notification_request → 信封解析（tenant/trace 恢复）→ Deliver 内核
// （模板渲染/频控/免打扰）→ message + outbound_log；消费幂等走 event_dedup 同事务去重。
// topic 由 tools/mqinit 预创建（E2 先建后用）；未配置 brokers 时不启动（仅 RPC 面）。
package consumer

import (
	"context"
	"encoding/json"

	"micro-server/services/notification/internal/config"
	"micro-server/services/notification/internal/logic"
	"micro-server/services/notification/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// MustStart 启动事件消费（未配置 brokers 时静默跳过；消费失败 DB 退避重投由 kq 承担）。
func MustStart(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("notification: 未配置 Kafka brokers，事件消费未启动（仅 RPC 面）")
		return
	}
	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.Group,
		Topic:      eventbus.TopicNotificationRequest,
		Offset:     "last",
		Conns:      1,
		Consumers:  1,
		Processors: 1,
	}, newHandler(sc))
	// kq 队列托管：Start 阻塞消费（内部自旋重连），挂后台 goroutine——
	// 服务主面是 RPC，不得被 MQ 消费阻塞启动（09:40 事故：q.Start 同步调用吞掉 s.Start()）。
	go q.Start()
	logx.Infof("notification: notification_request 消费已启动 group=%s", c.Kafka.Group)
}

// handler kq ConsumeHandler。
type handler struct {
	sc *svc.ServiceContext
}

func newHandler(sc *svc.ServiceContext) *handler { return &handler{sc: sc} }

// Consume 信封解包 → 租户恢复 → 去重 → Deliver 内核（02 §8 Subscriber 口径）。
func (h *handler) Consume(_ context.Context, _ /*key*/ string, val string) error {
	var env eventbus.Envelope
	if err := json.Unmarshal([]byte(val), &env); err != nil {
		logx.Errorf("notification: 信封解析失败（丢弃毒消息）: %v", err)
		return nil // 解析失败重投无意义，记日志跳过
	}
	ctx := context.Background()
	// 消费幂等：event_dedup 同事务去重（at-least-once → 业务幂等）
	dup, err := dedupOnce(ctx, h.sc, env.EventID)
	if err != nil {
		return err // DB 错误：交还 kq 退避重投
	}
	if dup {
		return nil
	}

	in, ok := logic.ParseEventPayload(env.Payload)
	if !ok {
		logx.Errorf("notification: payload 不合法 event=%s（丢弃）", env.EventID)
		return nil
	}
	// 事件信封携带 tenant_id（01 §7.3），消费侧恢复租户上下文
	out := logic.DeliverWithTenant(ctx, h.sc, env.TenantID, in)
	logx.Infof("notification: 事件投递 event=%s user=%d tpl=%s delivered=%v reason=%s",
		env.EventID, in.UserID, in.TemplateCode, out.Delivered, out.Reason)
	return nil
}

// dedupOnce 消费去重（event_dedup 表；唯一约束幂等）。
func dedupOnce(ctx context.Context, sc *svc.ServiceContext, eventID string) (bool, error) {
	var dup bool
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		res, err := session.ExecCtx(ctx,
			"insert ignore into `event_dedup` (`consumer_group`, `event_id`) values (?, ?)",
			"notification-deliver", eventID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		dup = n == 0
		return nil
	})
	return dup, err
}
