// query.go · 读模型服务面查询内核（GetReport/字典/watermark/重建）。

package logic

import (
	"context"
	"time"

	"micro-server/services/analytics/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetReportRows 报表查询内核：返回行（维度 + 指标键值，指标编码 = 字典 metric_code）。
// 通用化返回（map 结构）供 BFF 与导出统一消费；金额为字符串防精度丢失。
type ReportRow struct {
	StatDate    string            // 业务日（YYYY-MM-DD）
	StationID   int64             // 场站维度（sla/station 报表，0=平台级）
	WarehouseID int64             // 仓库维度（inventory 报表）
	Channel     string            // 支付渠道维度（finance 报表）
	Metrics     map[string]string // 指标键值（键 = 字典 metric_code；金额为字符串防精度丢失）
}

// GetReportRows 报表查询内核：按 reportType 分发到六大日聚合表扫描，
// 统一折成 ReportRow（维度 + 指标键值）供 BFF 与导出消费。
func GetReportRows(ctx context.Context, sc *svc.ServiceContext, tid int64, reportType, dateStart, dateEnd string, stationID, warehouseID int64, channel string) ([]*ReportRow, error) {
	start, err := parseDay(dateStart)
	if err != nil {
		return nil, errDateFormatBad
	}
	end, err := parseDay(dateEnd)
	if err != nil {
		return nil, errDateFormatBad
	}
	if end.Before(start) {
		return nil, errDateFormatBad.WithMsg("date_end 早于 date_start")
	}
	if !validReportType(reportType) || reportType == "event_ledger" {
		return nil, errReportTypeBad
	}

	switch reportType {
	case "sales":
		rows, err := sc.Models.Daily.ScanSales(ctx, tid, start, end)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate),
				Metrics: map[string]string{
					"orders_created": itoa64(r.OrdersCreated), "orders_paid": itoa64(r.OrdersPaid),
					"orders_cancelled": itoa64(r.OrdersCancelled), "orders_pay_timeout": itoa64(r.OrdersPayTimeout),
					"returns_approved": itoa64(r.ReturnsApproved), "shipments_signed": itoa64(r.ShipmentsSigned),
					"devices_shipped": itoa64(r.DevicesShipped), "gmv_paid": r.GmvPaid,
				},
			})
		}
		return out, nil
	case "inventory":
		rows, err := sc.Models.Daily.ScanInventory(ctx, tid, start, end, warehouseID)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate), WarehouseID: r.WarehouseId,
				Metrics: map[string]string{
					"stock_in_qty": itoa64(r.StockInQty), "stock_out_qty": itoa64(r.StockOutQty),
					"low_stock_events": itoa64(r.LowStockEvents),
				},
			})
		}
		return out, nil
	case "warranty":
		rows, err := sc.Models.Daily.ScanWarranty(ctx, tid, start, end)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate),
				Metrics: map[string]string{
					"warranties_started": itoa64(r.WarrantiesStarted), "warranties_device": itoa64(r.WarrantiesDevice),
					"warranties_station": itoa64(r.WarrantiesStation), "devices_activated": itoa64(r.DevicesActivated),
				},
			})
		}
		return out, nil
	case "sla":
		rows, err := sc.Models.Daily.ScanSla(ctx, tid, start, end, stationID)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate), StationID: r.StationId,
				Metrics: map[string]string{
					"alerts_total": itoa64(r.AlertsTotal), "alerts_p1": itoa64(r.AlertsP1),
					"alerts_p2": itoa64(r.AlertsP2), "alerts_p3": itoa64(r.AlertsP3),
				},
			})
		}
		return out, nil
	case "finance":
		rows, err := sc.Models.Daily.ScanFinance(ctx, tid, start, end, channel)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate), Channel: r.Channel,
				Metrics: map[string]string{
					"payments_count": itoa64(r.PaymentsCount), "paid_amount": r.PaidAmount,
					"refunds_count": itoa64(r.RefundsCount), "refund_amount": r.RefundAmount,
				},
			})
		}
		return out, nil
	case "station":
		rows, err := sc.Models.Daily.ScanStation(ctx, tid, start, end, stationID)
		if err != nil {
			return nil, err
		}
		out := make([]*ReportRow, 0, len(rows))
		for _, r := range rows {
			out = append(out, &ReportRow{
				StatDate: day(r.StatDate), StationID: r.StationId,
				Metrics: map[string]string{
					"stations_created": itoa64(r.StationsCreated), "station_alerts": itoa64(r.AlertsTotal),
					"alerts_p1": itoa64(r.AlertsP1),
				},
			})
		}
		return out, nil
	default:
		return nil, errReportTypeBad
	}
}

// RebuildReadModelCore 读模型全量重建（S8-01 消费组版本号机制）：
// 台账 + 六表 + watermark 清空，版本号 +1；重启后消费组名 analytics-read-v{N} 从头回放。
// Kafka 已过保留期的历史事件用 tools/p4backfill 按业务库补发。
func RebuildReadModelCore(ctx context.Context, sc *svc.ServiceContext) (string, error) {
	if err := sc.Models.Daily.TruncateAll(ctx); err != nil {
		return "", err
	}
	next, err := sc.Models.Meta.NextVersion(ctx)
	if err != nil {
		return "", err
	}
	logx.Infof("analytics: 读模型重建完成 version=%d（重启后新消费组生效；保留期外历史用 p4backfill 补）", next)
	return itoa64(next), nil
}

// StatDateOf 业务日导出（cronx 复用； occurred_at 日期口径）。
func StatDateOf(t time.Time) time.Time { return statDateOf(t) }
