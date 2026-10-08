// 逻辑层导出包装（consumer 包复用 Deliver 内核）+ confcenter 桥。

package logic

import (
	"context"

	"micro-server/services/notification/internal/confcenter"
	"micro-server/services/notification/internal/svc"
)

// DeliverWithTenant 事件/RPC 共用的租户显式入口（事件侧由信封恢复租户）。
func DeliverWithTenant(ctx context.Context, sc *svc.ServiceContext, tid int64, in deliverInput) deliverOutput {
	return deliver(ctx, sc, tid, in)
}

// ParseEventPayload 导出事件 payload 解析。
func ParseEventPayload(raw []byte) (deliverInput, bool) {
	return parseEventPayload(raw)
}

// confRateLimit 频控上限（confcenter 热调）。
func confRateLimit() int64 {
	return int64(confcenter.Current().RateLimitPer24h)
}
