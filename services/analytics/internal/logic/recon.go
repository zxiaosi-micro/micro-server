// recon.go · 日级对账内核（S8-04）：读模型 vs 业务库 → recon_diff + 差异站内信。
//
// 口径：业务库是唯一事实源；对账管理面直查业务库（报表服务面仍零 JOIN）。
// 差异修复只允许补投递重放（tools/p4backfill），禁止裸删（E10）。
// 指标覆盖以 report_metric_dict.recon=1 为准（状态边沿口径如 low_stock_events 不参与）。

package logic

import (
	"context"
	"time"

	"micro-server/services/analytics/internal/confcenter"
	"micro-server/services/analytics/internal/model"
	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// reconTarget 单个对账指标：source=业务库口径 model=读模型口径。
type reconTarget struct {
	metricCode string
	source     func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error)
	model      func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error)
}

// reconTargets 对账指标注册表（SQL 用库名限定表——跨库只读，A1 应用账号有 micro_% 授权）。
var reconTargets = []reconTarget{
	{
		metricCode: "orders_created",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_order.`order` WHERE tenant_id = ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "orders_created")
		},
	},
	{
		metricCode: "orders_paid",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_finance.payment WHERE tenant_id = ? AND status IN ('PAID','SETTLED') AND paid_at >= ? AND paid_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "orders_paid")
		},
	},
	{
		metricCode: "orders_cancelled",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_order.`order` WHERE tenant_id = ? AND status = 'CANCELLED' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "orders_cancelled")
		},
	},
	{
		metricCode: "orders_pay_timeout",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_order.`order` WHERE tenant_id = ? AND status = 'PAY_TIMEOUT' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "orders_pay_timeout")
		},
	},
	{
		metricCode: "returns_approved",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_order.return_order WHERE tenant_id = ? AND status IN ('APPROVED','REFUNDED') AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "returns_approved")
		},
	},
	{
		metricCode: "shipments_signed",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_order.shipment WHERE tenant_id = ? AND status = 'SIGNED' AND signed_at >= ? AND signed_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSalesMetric(ctx, tid, day, "shipments_signed")
		},
	},
	{
		metricCode: "stock_in_qty",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT IFNULL(SUM(qty),0) FROM micro_inventory.stock_record WHERE tenant_id = ? AND biz_type = 'STOCK_IN' AND created_at >= ? AND created_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumInventoryMetric(ctx, tid, day, "stock_in_qty")
		},
	},
	{
		metricCode: "stock_out_qty",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT IFNULL(SUM(qty),0) FROM micro_inventory.stock_record WHERE tenant_id = ? AND biz_type = 'DEDUCT' AND created_at >= ? AND created_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumInventoryMetric(ctx, tid, day, "stock_out_qty")
		},
	},
	{
		metricCode: "warranties_started",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_contract.warranty WHERE tenant_id = ? AND start_at >= ? AND start_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.CountWarrantyMetric(ctx, tid, day, "warranties_started")
		},
	},
	{
		metricCode: "devices_activated",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			// lifecycle 日志表 append-only，无 deleted_at
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_device.device_lifecycle_log WHERE tenant_id = ? AND event_type = 'device.activated' AND created_at >= ? AND created_at < ?")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.CountWarrantyMetric(ctx, tid, day, "devices_activated")
		},
	},
	{
		metricCode: "payments_count",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_finance.payment WHERE tenant_id = ? AND status IN ('PAID','SETTLED') AND paid_at >= ? AND paid_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			payments, _, _, _, err := sc.Models.Daily.SumFinanceRows(ctx, tid, day)
			return payments, err
		},
	},
	{
		metricCode: "refunds_count",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_finance.refund WHERE tenant_id = ? AND status = 'SUCCESS' AND updated_at >= ? AND updated_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			_, _, refunds, _, err := sc.Models.Daily.SumFinanceRows(ctx, tid, day)
			return refunds, err
		},
	},
	{
		metricCode: "alerts_total",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			// ops alert 无软删列（追加式）
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_ops.alert WHERE tenant_id = ? AND first_seen_at >= ? AND first_seen_at < ?")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSlaMetric(ctx, tid, day, "alerts_total")
		},
	},
	{
		metricCode: "alerts_p1",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_ops.alert WHERE tenant_id = ? AND level = 'P1' AND first_seen_at >= ? AND first_seen_at < ?")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.SumSlaMetric(ctx, tid, day, "alerts_p1")
		},
	},
	{
		metricCode: "stations_created",
		source: func(ctx context.Context, sc *svc.ServiceContext, tid int64, dayStart, dayEnd time.Time) (int64, error) {
			return countBusiness(sc, ctx, tid, dayStart, dayEnd,
				"SELECT COUNT(*) FROM micro_station.station WHERE tenant_id = ? AND created_at >= ? AND created_at < ? AND deleted_at IS NULL")
		},
		model: func(ctx context.Context, sc *svc.ServiceContext, tid int64, day time.Time) (int64, error) {
			return sc.Models.Daily.CountStationMetric(ctx, tid, day, "stations_created")
		},
	},
}

func countBusiness(sc *svc.ServiceContext, ctx context.Context, tid int64, dayStart, dayEnd time.Time, query string) (int64, error) {
	var n int64
	err := sc.Conn.QueryRowCtx(ctx, &n, query, tid, dayStart, dayEnd)
	return n, err
}

// RunReconCore 对账执行内核（RPC RunRecon 与 cron daily_recon 共用）。
// 幂等：recon_state 防并发重复（RUNNING 时拒绝）；recon_diff uk 幂等；差异消除自动关单。
func RunReconCore(ctx context.Context, sc *svc.ServiceContext, bizDate time.Time) (int64, error) {
	// 防重入：RUNNING 中拒绝（cron 与手动触发可能并发）
	if state, err := sc.Models.Recon.GetState(ctx, bizDate); err == nil && state.Status == "RUNNING" {
		return state.DiffTotal, errReconRunning
	}
	if err := sc.Models.Recon.UpsertState(ctx, bizDate, "RUNNING", 0); err != nil {
		return 0, err
	}

	dayEnd := bizDate.AddDate(0, 0, 1)
	tenants, err := reconTenantIDs(ctx, sc, bizDate, dayEnd)
	if err != nil {
		_ = sc.Models.Recon.UpsertState(ctx, bizDate, "PENDING", 0)
		return 0, err
	}

	diffCount := 0
	for _, tid := range tenants {
		for _, target := range reconTargets {
			source, err := target.source(ctx, sc, tid, bizDate, dayEnd)
			if err != nil {
				logx.Errorf("analytics: 对账 source 查询失败 tenant=%d metric=%s: %v", tid, target.metricCode, err)
				continue // 单指标失败不阻塞整轮（下轮补查）
			}
			mdl, err := target.model(ctx, sc, tid, bizDate)
			if err != nil {
				logx.Errorf("analytics: 对账 model 查询失败 tenant=%d metric=%s: %v", tid, target.metricCode, err)
				continue
			}
			diff := source - mdl
			if diff != 0 {
				diffCount++
				if uerr := sc.Models.Recon.UpsertDiff(ctx, &model.ReconDiff{
					TenantId:    tid,
					ReconDate:   bizDate, // recon_date 记录对账所属业务日（口径与 biz_date 一致）
					BizDate:     bizDate,
					MetricCode:  target.metricCode,
					SourceCount: source,
					ModelCount:  mdl,
					DiffCount:   diff,
				}); uerr != nil {
					logx.Errorf("analytics: 差异登记失败 tenant=%d metric=%s: %v", tid, target.metricCode, uerr)
					continue
				}
				logx.Infof("analytics: 对账差异 tenant=%d metric=%s source=%d model=%d diff=%d",
					tid, target.metricCode, source, mdl, diff)
			} else {
				// 已对平：自动关单（此前登记的差异补齐 → RESOLVED）
				if rerr := sc.Models.Recon.ResolveDiff(ctx, tid, target.metricCode, bizDate); rerr != nil {
					logx.Errorf("analytics: 差异关单失败 tenant=%d metric=%s: %v", tid, target.metricCode, rerr)
				}
			}
		}
	}

	openTotal, err := sc.Models.Recon.CountOpen(ctx, bizDate)
	if err != nil {
		return 0, err
	}
	if err := sc.Models.Recon.UpsertState(ctx, bizDate, "DONE", openTotal); err != nil {
		return 0, err
	}

	// 差异站内信（notification_request 事件 → notification 服务落站内信；每业务日防重）
	if openTotal > 0 {
		if serr := notifyReconDiff(ctx, sc, bizDate, openTotal); serr != nil {
			logx.Errorf("analytics: 对账站内信发送失败 biz_date=%s: %v（下轮重试）", bizDate.Format("2006-01-02"), serr)
		}
	}
	return openTotal, nil
}

// reconTenantIDs 参与对账的租户（两侧并集：业务库当日有数据 或 读模型当日有台账）。
func reconTenantIDs(ctx context.Context, sc *svc.ServiceContext, dayStart, dayEnd time.Time) ([]int64, error) {
	var ids []int64
	err := sc.Conn.QueryRowsCtx(ctx, &ids, `
SELECT DISTINCT tenant_id FROM (
    SELECT tenant_id FROM micro_order.`+"`order`"+` WHERE created_at >= ? AND created_at < ? AND deleted_at IS NULL
    UNION
    SELECT tenant_id FROM micro_inventory.stock_record WHERE created_at >= ? AND created_at < ? AND deleted_at IS NULL
    UNION
    SELECT tenant_id FROM micro_ops.alert WHERE first_seen_at >= ? AND first_seen_at < ?
    UNION
    SELECT tenant_id FROM report_event_entry WHERE stat_date = ?
) t WHERE tenant_id > 0`,
		dayStart, dayEnd, dayStart, dayEnd, dayStart, dayEnd, dayStart)
	return ids, err
}

// notifyReconDiff 差异站内信（Outbox 单事件：recon_state.notified 防重兜底）。
func notifyReconDiff(ctx context.Context, sc *svc.ServiceContext, bizDate time.Time, diffTotal int64) error {
	state, err := sc.Models.Recon.GetState(ctx, bizDate)
	if err == nil && state.Notified == 1 {
		return nil // 已通知
	}

	// 收件人：confcenter 覆盖 > 启动配置 > 平台管理员 seed uid
	target := confTargetUID(sc)
	day := bizDate.Format("2006-01-02")
	payload := map[string]any{
		"user_id":       target,
		"template_code": "RECON_DIFF",
		"params": map[string]string{
			"biz_date":   day,
			"diff_total": itoa64(diffTotal),
		},
		"biz_type": "recon",
		"biz_id":   day,
	}
	if err := eventbus.Emit(ctx, sc.Conn, eventbus.EmitInput{
		Topic:    eventbus.TopicNotificationRequest,
		Type:     eventbus.TypeNotificationRequest,
		Key:      "recon:" + day,
		TenantID: 9000000000000000001, // 平台租户（差异站内信发平台管理员）
		Payload:  payload,
	}); err != nil {
		return err
	}
	return sc.Models.Recon.MarkNotified(ctx, bizDate)
}

// confTargetUID 站内信收件人：confcenter 热调 > 启动配置 > 平台管理员 seed uid。
func confTargetUID(sc *svc.ServiceContext) int64 {
	if v := confcenter.Current().ReconTargetUID; v > 0 {
		return v
	}
	if sc.Config.ReconTargetUID > 0 {
		return sc.Config.ReconTargetUID
	}
	return 9000000000000000002 // seed 平台管理员（tools/seed 固定 uid）
}
