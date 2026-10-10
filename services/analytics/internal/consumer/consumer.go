// Package consumer · analytics 事件消费（S8）。
// 双消费组：读模型 analytics-read-v{N}（14 类日聚合事件，版本号进组名——重建时 +1 从头回放）
// 与导出 analytics-export（export_requested 渲染管道）。组名全局唯一（E2），Offset first + 去重兜底重放。
package consumer

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/analytics/internal/config"
	"micro-server/services/analytics/internal/logic"
	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// readTopics 读模型订阅清单（14 类事件，S8-01 正本；E2 先建后用——tools/mqinit 预创建）。
var readTopics = []string{
	eventbus.TopicOrderCreated,
	eventbus.TopicOrderPaid,
	eventbus.TopicOrderCancelled,
	eventbus.TopicOrderPayTimeout,
	eventbus.TopicStockIn,
	eventbus.TopicStockOut,
	eventbus.TopicStockLow,
	eventbus.TopicPaymentRefunded,
	eventbus.TopicOrderReturnApproved,
	eventbus.TopicWarrantyStarted,
	eventbus.TopicDeviceActivated,
	eventbus.TopicShipmentSigned,
	eventbus.TopicStationCreated,
	eventbus.TopicAlertCreated,
}

// MustStart 启动事件消费（未配置 brokers 时静默跳过；消费失败 DB 退避重投由 Subscriber 承担）。
func MustStart(c config.Config, sc *svc.ServiceContext) {
	if len(c.Kafka.Brokers) == 0 {
		logx.Info("analytics: 未配置 Kafka brokers，事件消费未启动")
		return
	}

	// 读模型组：版本号后缀（report_meta.rebuild_version；重建 = 版本+1 + 重启 → 新组从头回放）
	version, err := sc.Models.Meta.GetVersion(context.Background())
	if err != nil {
		logx.Errorf("analytics: 读模型版本读取失败（用 v1）: %v", err)
		version = "1"
	}
	readGroup := fmt.Sprintf("%s-v%s", c.Kafka.Group, version)
	readSub, err := eventbus.NewSubscriber(sc.Conn, readHandler(sc), eventbus.SubConf{Group: readGroup})
	if err != nil {
		logx.Must(err)
	}
	for _, topic := range readTopics {
		q := kq.MustNewQueue(kq.KqConf{
			Brokers:    c.Kafka.Brokers,
			Group:      readGroup,
			Topic:      topic,
			Offset:     "first", // 新组从头消费；event_dedup + 台账业务键幂等兜底重放（known-pit：last 会漏消息）
			Conns:      1,
			Consumers:  1,
			Processors: 1,
		}, kqHandler{sub: readSub}, kq.WithMaxWait(200*time.Millisecond)) // 低水位 topic 及时取件（默认 1s MaxWait 抖动大）
		// q.Start() 是阻塞调用：必须 go 化（09:40 事故）
		go q.Start()
	}

	// 导出组：export_requested 渲染管道（S8-03）
	exportSub, err := eventbus.NewSubscriber(sc.Conn, exportHandler(sc), eventbus.SubConf{Group: c.Kafka.ExportGroup})
	if err != nil {
		logx.Must(err)
	}
	q := kq.MustNewQueue(kq.KqConf{
		Brokers:    c.Kafka.Brokers,
		Group:      c.Kafka.ExportGroup,
		Topic:      eventbus.TopicExportRequested,
		Offset:     "first",
		Conns:      1,
		Consumers:  1,
		Processors: 1,
	}, kqHandler{sub: exportSub}, kq.WithMaxWait(200*time.Millisecond))
	go q.Start()

	logx.Infof("analytics: 事件消费已启动 read_group=%s topics=%d export_group=%s",
		readGroup, len(readTopics), c.Kafka.ExportGroup)
}

// readHandler 读模型 handler（Subscriber 提供 dedup/trace/租户恢复）。
func readHandler(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		return logic.ConsumeReadEvent(ctx, sc, "", env)
	}
}

// exportHandler 导出 handler。
func exportHandler(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		return logic.ConsumeExportEvent(ctx, sc, env)
	}
}

// kqHandler 适配 eventbus.Subscriber（kq.ConsumeHandler）。
type kqHandler struct{ sub *eventbus.Subscriber }

func (h kqHandler) Consume(_ context.Context, key, val string) error {
	return h.sub.Consume(key, val)
}
