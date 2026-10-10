// readmodel.go · 读模型消费内核（S8-01）：14 类事件 → 业务键幂等台账 → 六大日聚合 + watermark。
//
// 幂等三层：①event_dedup（event_id，eventbus.Subscriber 同事务）②report_event_entry uk_biz
// （event_type+biz_key——补投递重放新 event_id 也会被挡）③聚合 upsert 只在 ①② 首次时执行。
// handler 失败不阻塞：Subscriber 失败 → event_retry 退避重投 ≤16 → dead 表（对账 + p4backfill 兜底）。
package logic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"micro-server/services/analytics/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// readHandlers 14 类事件 → 消费内核表（event_type 分流正本）。
// 其余事件（notification.request/audit.event/export.requested/cmd.ack/cmd.failed/ota.paused/device.anomaly）
// 不进日聚合：分别由 notification/audit/analytics-export 消费或属过程性事件。
func applyReadEvent(ctx context.Context, sc readDeps, env *eventbus.Envelope) error {
	if env.TenantID <= 0 {
		// 无租户事件（理论不应出现：14 类事件全部业务域发出）——按毒消息跳过，不重投。
		logx.Errorf("analytics: 事件无租户跳过 event=%s type=%s", env.EventID, env.EventType)
		return nil
	}
	statDate := statDateOf(env.OccurredAt)
	payload := payloadOf(env)

	// 业务幂等键（与 payload 结构强耦合——payload 正本见各服务 logic/events.go，改动需对齐）
	bizKey, err := bizKeyOf(env.EventType, payload, env.EventID)
	if err != nil {
		return err
	}

	err = sc.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// 1. 台账（uk_biz 幂等：重复 → ErrDupBiz 整单跳过，聚合不动）
		entry := &model.EventEntry{
			EventID:   env.EventID,
			EventType: env.EventType,
			Topic:     topicOf(env.EventType),
			TenantId:  env.TenantID,
			BizKey:    bizKey,
			StatDate:  statDate,
			Payload:   string(payload),
		}
		if err := sc.models.Entry.InsertInTx(ctx, session, entry); err != nil {
			if isDupKey(err) {
				return errDupBiz
			}
			return err
		}
		// 2. 聚合增量
		if err := applyAggregate(ctx, session, sc, env, statDate); err != nil {
			return err
		} // 3. watermark 同事务推进
		return sc.models.Watermark.BumpInTx(ctx, session, topicOf(env.EventType), sc.readGroup, env.EventID, statDate)
	})
	if err == errDupBiz {
		logx.Infof("analytics: 业务键重复跳过 type=%s biz=%s event=%s", env.EventType, bizKey, env.EventID)
		return nil
	}
	return err
}

// applyAggregate 事件 payload → 聚合增量（与各域事件契约 1:1，S8-01 14 类）。
func applyAggregate(ctx context.Context, session sqlx.Session, sc readDeps, env *eventbus.Envelope, date time.Time) error {
	tid := env.TenantID
	payload := payloadOf(env)
	switch env.EventType {
	case eventbus.TypeOrderCreated:
		return sc.models.Daily.ApplyOrderCreatedTx(ctx, session, tid, date)
	case eventbus.TypeOrderPaid:
		var p struct {
			Channel    string `json:"channel"`
			PaidAmount string `json:"paid_amount"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("order.paid payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyOrderPaidTx(ctx, session, tid, date, p.Channel, p.PaidAmount)
	case eventbus.TypeOrderCancelled:
		return sc.models.Daily.ApplyOrderCancelledTx(ctx, session, tid, date)
	case eventbus.TypeOrderPayTimeout:
		return sc.models.Daily.ApplyOrderPayTimeoutTx(ctx, session, tid, date)
	case eventbus.TypeOrderReturnApproved:
		return sc.models.Daily.ApplyReturnApprovedTx(ctx, session, tid, date)
	case eventbus.TypeShipmentSigned:
		var p struct {
			Sns []string `json:"sns"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("shipment.signed payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyShipmentSignedTx(ctx, session, tid, date, int64(len(p.Sns)))
	case eventbus.TypeStockIn:
		var p struct {
			WarehouseID int64 `json:"warehouse_id"`
			Qty         int64 `json:"qty"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("stock.in payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyStockInTx(ctx, session, tid, date, p.WarehouseID, p.Qty)
	case eventbus.TypeStockOut:
		var p struct {
			WarehouseID int64 `json:"warehouse_id"`
			Qty         int64 `json:"qty"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("stock.out payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyStockOutTx(ctx, session, tid, date, p.WarehouseID, p.Qty)
	case eventbus.TypeStockLow:
		var p struct {
			WarehouseID int64 `json:"warehouse_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("stock.low payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyStockLowTx(ctx, session, tid, date, p.WarehouseID)
	case eventbus.TypeWarrantyStarted:
		var p struct {
			Level string `json:"level"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("warranty.started payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyWarrantyStartedTx(ctx, session, tid, date, p.Level)
	case eventbus.TypeDeviceActivated:
		return sc.models.Daily.ApplyDeviceActivatedTx(ctx, session, tid, date)
	case eventbus.TypeStationCreated:
		var p struct {
			StationID int64 `json:"station_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("station.created payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyStationCreatedTx(ctx, session, tid, date, p.StationID)
	case eventbus.TypeAlertCreated:
		var p struct {
			StationID int64  `json:"station_id"`
			Level     string `json:"level"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("alert.created payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyAlertCreatedTx(ctx, session, tid, date, p.StationID, p.Level)
	case eventbus.TypePaymentRefunded:
		var p struct {
			Channel string `json:"channel"`
			Amount  string `json:"amount"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("payment.refunded payload 解析失败: %w", err)
		}
		return sc.models.Daily.ApplyPaymentRefundedTx(ctx, session, tid, date, p.Channel, p.Amount)
	default:
		// 未登记事件类型：记日志跳过（不重投不告警——读模型只认 14 类正本）。
		logx.Infof("analytics: 未登记事件类型跳过 type=%s event=%s", env.EventType, env.EventID)
		return nil
	}
}

// bizKeyOf 业务幂等键提取（读模型契约正本；stock.low 为状态边沿事件无业务键 → event_id）。
func bizKeyOf(eventType string, payload json.RawMessage, eventID string) (string, error) {
	switch eventType {
	case eventbus.TypeOrderCreated, eventbus.TypeOrderPaid, eventbus.TypeOrderCancelled,
		eventbus.TypeOrderPayTimeout:
		var p struct {
			OrderNo string `json:"order_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("%s biz_key 提取失败: %w", eventType, err)
		}
		return p.OrderNo, nil
	case eventbus.TypeOrderReturnApproved:
		var p struct {
			ReturnNo string `json:"return_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.ReturnNo, nil
	case eventbus.TypePaymentRefunded:
		var p struct {
			RefundNo string `json:"refund_no"`
			OrderNo  string `json:"order_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		if p.RefundNo != "" {
			return p.RefundNo, nil // 多笔部分退款：refund_no 才是业务键
		}
		return p.OrderNo, nil
	case eventbus.TypeStockIn, eventbus.TypeStockOut:
		var p struct {
			BizNo string `json:"biz_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.BizNo, nil
	case eventbus.TypeStockLow:
		return eventID, nil // 边沿事件无稳定业务键：以 event_id 幂等（对账不覆盖，见指标字典）
	case eventbus.TypeWarrantyStarted:
		var p struct {
			WarrantyNo string `json:"warranty_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.WarrantyNo, nil
	case eventbus.TypeDeviceActivated:
		var p struct {
			Sn string `json:"sn"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.Sn, nil
	case eventbus.TypeShipmentSigned:
		var p struct {
			ShipmentNo string `json:"shipment_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.ShipmentNo, nil
	case eventbus.TypeStationCreated:
		var p struct {
			StationNo string `json:"station_no"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return p.StationNo, nil
	case eventbus.TypeAlertCreated:
		var p struct {
			AlertID int64 `json:"alert_id"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return "", fmt.Errorf("biz_key 提取失败: %w", err)
		}
		return strconv.FormatInt(p.AlertID, 10), nil
	default:
		return "", nil
	}
}

// payloadOf payload 归一化（known-pit 兼容）：信封 Payload 为 JSON 对象时原样返回；
// 为 JSON 字符串（base64——历史版本 Emit 预序列化 []byte 的产物）时解码出原始 JSON。
func payloadOf(env *eventbus.Envelope) []byte {
	raw := []byte(env.Payload)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var b64 string
		if err := json.Unmarshal(trimmed, &b64); err == nil {
			if dec, derr := base64.StdEncoding.DecodeString(b64); derr == nil && len(dec) > 0 {
				return dec
			}
		}
	}
	return raw
}

// statDateOf 业务日（信封 occurred_at 日期，本地时区——与业务库 created_at 同钟）。
func statDateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func topicOf(eventType string) string {
	switch eventType {
	case eventbus.TypeOrderCreated:
		return eventbus.TopicOrderCreated
	case eventbus.TypeOrderPaid:
		return eventbus.TopicOrderPaid
	case eventbus.TypeOrderCancelled:
		return eventbus.TopicOrderCancelled
	case eventbus.TypeOrderPayTimeout:
		return eventbus.TopicOrderPayTimeout
	case eventbus.TypeStockIn:
		return eventbus.TopicStockIn
	case eventbus.TypeStockOut:
		return eventbus.TopicStockOut
	case eventbus.TypeStockLow:
		return eventbus.TopicStockLow
	case eventbus.TypePaymentRefunded:
		return eventbus.TopicPaymentRefunded
	case eventbus.TypeOrderReturnApproved:
		return eventbus.TopicOrderReturnApproved
	case eventbus.TypeWarrantyStarted:
		return eventbus.TopicWarrantyStarted
	case eventbus.TypeDeviceActivated:
		return eventbus.TopicDeviceActivated
	case eventbus.TypeShipmentSigned:
		return eventbus.TopicShipmentSigned
	case eventbus.TypeStationCreated:
		return eventbus.TopicStationCreated
	case eventbus.TypeAlertCreated:
		return eventbus.TopicAlertCreated
	default:
		return "unknown"
	}
}
