// cron 推进器（ADR-09）：①支付超时扫描 ②Saga 重试扫描。注册进 cronx 注册表（E16：last_run 指标）。
// ③超限人工队列 = saga.status=MANUAL（GetSaga 可视化 + RetrySaga 人工重推）。

package logic

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// ScanPayTimeout ①支付超时扫描：pay_expire_at 到期 → 取消 + 释放库存 + order_pay_timeout 事件。
// 全程无 MQ 延迟消息（ADR-09 阶段验收项）。
func ScanPayTimeout(ctx context.Context, sc *svc.ServiceContext) error {
	expired, err := sc.Models.Order.FindPayExpired(ctx, time.Now(), advanceBatchOf(sc))
	if err != nil {
		return err
	}
	for _, o := range expired {
		// 按订单租户重建 ctx（下游释放库存 RPC 需要租户元数据；E5 不挂请求 ctx）
		octx := tenantxWith(context.WithoutCancel(context.Background()), o.TenantId)
		reason := fmt.Sprintf("支付超时自动取消（超过 %d 分钟未支付）", effectivePayTimeoutMin(sc))
		if err := cancelOrderInternal(octx, sc, o.TenantId, o.OrderNo, reason, "PAY_TIMEOUT", "PAY_TIMEOUT"); err != nil {
			logx.WithContext(ctx).Errorf("支付超时取消失败 order_no=%s: %v", o.OrderNo, err)
		}
	}
	return nil
}

// CountManualSaga 人工队列积压计数（监控指标口径：saga_stuck_orders）。
func CountManualSaga(ctx context.Context, sc *svc.ServiceContext) int64 {
	ctx = tenantxSkipCtx(ctx)
	n, err := sc.Models.Saga.CountByStatus(ctx, "MANUAL")
	if err != nil {
		return 0
	}
	return n
}

// timeFromMilli 0 值兜底为当前时刻。
func timeFromMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Now()
	}
	return time.UnixMilli(ms)
}
