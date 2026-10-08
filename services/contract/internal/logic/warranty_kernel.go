// 质保内核（S5-04，FR-CTR-005）：事件驱动起算 + 随单建立。
//
// 起算事件（02 §9.1 / FR-ORD-005）：
//   - shipment_signed（发货签收）→ 按 SN 起算 DEVICE 层质保；无 SN 明细则不起算（等激活）；
//   - device_activated（设备激活，S6 事件）→ 兜底起算（签收早于激活或无签收链路）。
//
// start_rule 快照（策略冻结）：{"event":"shipment_signed|device_activated","months":24}——
// 起算依据落库可追溯；到期扫描（cron）置 EXPIRED。
// 起算成功发 warranty_started 事件（notification/ops 消费口径）。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"micro-server/services/contract/internal/confcenter"
	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// StartRule 起算规则快照（warranty.start_rule JSON）。
type StartRule struct {
	Event  string `json:"event"`           // shipment_signed / device_activated
	Months int32  `json:"months"`          // 质保月数
}

// CreateWarrantiesFromOrder 随单建立质保计划（Saga 步骤5 CreateFromOrder 一部分）。
// DEVICE 层：per SN 一条（PENDING 待起算）；场站单再建 STATION 层一条（target_key=order_no 暂替，S6 建站回填）。
func CreateWarrantiesFromOrder(ctx context.Context, sc *svc.ServiceContext, tid int64,
	session sqlx.Session, orderNo, orderType string, itemsJSON string) (int, error) {

	type itemVO struct {
		SkuId int64  `json:"sku_id"`
		Sn    string `json:"sn,omitempty"`
		Qty   int32  `json:"qty"`
	}
	var items []itemVO
	if itemsJSON != "" {
		if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
			return 0, fmt.Errorf("items_json 解析失败: %w", err)
		}
	}

	months := confcenter.Current().DefaultWarrantyMonths
	rule := mustJSON(StartRule{Event: "shipment_signed", Months: int32(months)})
	ruleActivate := mustJSON(StartRule{Event: "device_activated", Months: int32(months)})

	count := 0
	for _, it := range items {
		if it.Sn == "" {
			continue // 无 SN 的普通货物不建质保（备件/耗材）
		}
		w := &model.Warranty{
			WarrantyId: sc.Snowflake.MustNextID(), WarrantyNo: "WAR" + fmt.Sprint(sc.Snowflake.MustNextID()),
			Level: "DEVICE", TargetType: "DEVICE",
			TargetKey: it.Sn, Status: "PENDING",
			Months: int64(months), StartRule: sqlString(ruleActivate),
			SourceType: "ORDER", SourceNo: sqlString(orderNo),
			TenantId: tid,
		}
		// 基线：签收起算（FR-ORD-005 验收口径——签收事件驱动质保起算按策略）
		w.StartRule = sqlString(rule)
		if err := sc.Models.Warranty.InsertTx(ctx, session, w); err != nil {
			if isDupKey(err) {
				continue // 重放幂等
			}
			return 0, err
		}
		count++
	}

	// 场站单：STATION 层一条（双层质保，FR-CTR-005；S6 建站后 target 回填）
	if orderType == "STATION" {
		w := &model.Warranty{
			WarrantyId: sc.Snowflake.MustNextID(), WarrantyNo: "WAR" + fmt.Sprint(sc.Snowflake.MustNextID()),
			Level: "STATION", TargetType: "STATION",
			TargetKey: orderNo, Status: "PENDING",
			Months: int64(months), StartRule: sqlString(rule),
			SourceType: "ORDER", SourceNo: sqlString(orderNo),
			TenantId: tid,
		}
		if err := sc.Models.Warranty.InsertTx(ctx, session, w); err != nil && !isDupKey(err) {
			return 0, err
		}
		count++
	}
	return count, nil
}

// HandleShipmentSigned shipment_signed（order 发出）→ 按策略快照起算 DEVICE 层质保。
func HandleShipmentSigned(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			OrderNo    string   `json:"order_no"`
			ShipmentNo string   `json:"shipment_no"`
			Sns        []string `json:"sns"`
			SignedBy   string   `json:"signed_by"`
			SignedAt   int64    `json:"signed_at"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("shipment_signed 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		started, err := startWarranties(ctx, sc, tid, "shipment_signed", p.Sns, p.SignedAt)
		if err != nil {
			return err
		}
		logx.WithContext(ctx).Infof("shipment_signed 质保起算 order_no=%s sns=%v started=%d", p.OrderNo, p.Sns, started)
		return nil
	}
}

// HandleDeviceActivated device_activated（S6 设备域发出）→ 兜底起算（签收早于激活或无签收链路）。
func HandleDeviceActivated(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			Sn        string `json:"sn"`
			ActivatedAt int64 `json:"activated_at"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("device_activated 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		if p.Sn == "" {
			logx.WithContext(ctx).Errorf("device_activated 缺 sn（跳过）")
			return nil
		}
		_, err := startWarranties(ctx, sc, tid, "device_activated", []string{p.Sn}, p.ActivatedAt)
		if err != nil {
			return err
		}
		logx.WithContext(ctx).Infof("device_activated 质保兜底起算 sn=%s", p.Sn)
		return nil
	}
}

// startWarranties 按 SN 起算（规则快照 event 匹配才起算；起算成功发 warranty_started）。
func startWarranties(ctx context.Context, sc *svc.ServiceContext, tid int64,
	ruleEvent string, sns []string, atMilli int64) (int, error) {

	startAt := time.Now()
	if atMilli > 0 {
		startAt = time.UnixMilli(atMilli)
	}
	started := 0
	for _, sn := range sns {
		list, err := sc.Models.Warranty.FindByTarget(ctx, tid, "DEVICE", 0, sn)
		if err != nil {
			return started, err
		}
		for _, w := range list {
			if w.Status != "PENDING" {
				continue // 已起算/已退款/已到期：幂等跳过
			}
			var rule StartRule
			if raw := nullStr(w.StartRule); raw != "" {
				_ = json.Unmarshal([]byte(raw), &rule)
			}
			if rule.Event != "" && rule.Event != ruleEvent {
				continue // 规则快照指定的起算事件不匹配（如须等签收而激活先到）
			}
			months := w.Months
			if rule.Months > 0 {
				months = int64(rule.Months)
			}
			endAt := startAt.AddDate(0, int(months), 0)
			err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
				if err := sc.Models.Warranty.StartTx(ctx, session, tid, w.WarrantyId, startAt, endAt); err != nil {
					return err
				}
				// warranty_started（notification 通知/audit 留痕消费）
				return eventbus.Emit(ctx, session, warrantyStartedEvent(tid, w, startAt, endAt))
			})
			if err != nil {
				if err == model.ErrStatusConflict {
					continue // 并发已起算：幂等
				}
				return started, err
			}
			started++
		}
	}
	return started, nil
}

// warrantyStartedPayload warranty_started 事件体。
type warrantyStartedPayload struct {
	TenantID   int64  `json:"tenant_id"`
	WarrantyNo string `json:"warranty_no"`
	Level      string `json:"level"`
	TargetKey  string `json:"target_key"`
	StartAt    int64  `json:"start_at"` // UnixMilli
	EndAt      int64  `json:"end_at"`   // UnixMilli
	Months     int64  `json:"months"`
}

func warrantyStartedEvent(tid int64, w *model.Warranty, startAt, endAt time.Time) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicWarrantyStarted, Type: eventbus.TypeWarrantyStarted,
		Key: w.WarrantyNo, TenantID: tid,
		Payload: warrantyStartedPayload{
			TenantID: tid, WarrantyNo: w.WarrantyNo, Level: w.Level,
			TargetKey: w.TargetKey, StartAt: startAt.UnixMilli(), EndAt: endAt.UnixMilli(), Months: w.Months,
		},
		EventID: eventbus.NewEventID(),
	}
}
