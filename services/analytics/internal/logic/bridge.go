// bridge.go · 消费入口（consumer 包调用的导出内核）。

package logic

import (
	"context"
	"encoding/json"

	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// ConsumeReadEvent 读模型消费内核（eventbus.Subscriber handler——dedup/重试/死信由 Subscriber 承担）。
// 返回 error 交还 Subscriber 记 retry；errDupBiz 类幂等跳过已在内核内消化（返回 nil）。
func ConsumeReadEvent(ctx context.Context, sc *svc.ServiceContext, group string, env *eventbus.Envelope) error {
	return applyReadEvent(ctx, readDeps{conn: sc.Conn, models: sc.Models, readGroup: group}, env)
}

// ConsumeExportEvent 导出消费内核（export_requested → 渲染调度，S8-03）。
func ConsumeExportEvent(ctx context.Context, sc *svc.ServiceContext, env *eventbus.Envelope) error {
	var p struct {
		TaskID int64 `json:"task_id"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil || p.TaskID == 0 {
		// 畸形 payload：记日志后吞掉（不重投——重投也是同结果，dead 表有轨迹可查）。
		logx.Errorf("analytics: export_requested payload 不合法 event=%s: %v", env.EventID, err)
		return nil
	}
	scheduleExport(ctx, sc, p.TaskID)
	return nil
}

// ParseEnvelope 消费侧信封解析（consumer 包复用 eventbus.ParseEnvelope）。
func ParseEnvelope(raw string) (*eventbus.Envelope, error) {
	return eventbus.ParseEnvelope(raw)
}
