// 事件消费桥（S7-01）：alert_candidate / cmd_failed / ota_paused / device_anomaly →
// ingestAlert 判定聚合；station_created → 自动建巡检计划。
// 幂等 = event_dedup 同事务去重 + 业务语义幂等（抑制窗口聚合/计划查回）。
package logic

import (
	"context"
	"encoding/json"
	"time"

	"micro-server/services/ops/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// ConsumeEvent 信封事件统一入口（topic 由 consumer 队列绑定传入）。
func ConsumeEvent(ctx context.Context, sc *svc.ServiceContext, topic string, env eventbus.Envelope) error {
	// 消费幂等：event_dedup 同事务去重（at-least-once → 业务幂等兜底）
	dup, err := dedupOnce(ctx, sc, env.EventID)
	if err != nil {
		return err // DB 错误：交还 kq 退避重投
	}
	if dup {
		return nil
	}

	// 事件信封携带 tenant_id（01 §7.3），消费侧恢复租户上下文
	ctx = tenantx.WithTenant(ctx, env.TenantID)
	switch topic {
	case eventbus.TopicCmdFailed:
		return consumeCmdFailed(ctx, sc, env)
	case eventbus.TopicOtaPaused:
		return consumeOtaPaused(ctx, sc, env)
	case eventbus.TopicDeviceAnomaly:
		return consumeDeviceAnomaly(ctx, sc, env)
	case eventbus.TopicStationCreated:
		return consumeStationCreated(ctx, sc, env)
	default:
		logx.Errorf("ops: 未注册 topic 消费 %s（丢弃）", topic)
		return nil
	}
}

// ConsumeCandidateRaw alert_candidate 裸 payload（iotingest rules.go 直推，无信封）。
// 幂等交给抑制窗口聚合（同源重复候选 → AggHit 累计，不重复开告警）。
func ConsumeCandidateRaw(ctx context.Context, sc *svc.ServiceContext, raw []byte) error {
	var p candidatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		logx.Errorf("ops: alert_candidate payload 不合法（丢弃）: %v", err)
		return nil
	}
	ingestAlert(sc, alertIngest{
		TenantId:   p.TenantId,
		Source:     "TELEMETRY",
		Metric:     p.Metric,
		Comparator: p.Comparator,
		Threshold:  &p.Threshold,
		LastValue:  &p.Value,
		Level:      p.Level,
		Sn:         p.Sn,
		Title:      p.Metric + " 越限（" + p.Sn + "）",
		Content:    "指标 " + p.Metric + " 当前值 " + formatF(p.Value) + " " + p.Comparator + " 阈值 " + formatF(p.Threshold),
		OccurredAt: msTime(p.OccurredAt),
	})
	return nil
}

// candidatePayload iotingest rules.go 同构载荷（契约对齐，FR-IOT-009）。
type candidatePayload struct {
	TenantId   int64   `json:"tenant_id"`   // 租户 ID（iotingest 直推裸 JSON 无信封，以此定位租户）
	Sn         string  `json:"sn"`          // 设备 SN（告警聚合键）
	ProductKey string  `json:"product_key"` // 产品标识
	Metric     string  `json:"metric"`      // 越限指标（soc/voltage/current/temperature/power）
	Value      float64 `json:"value"`       // 越限瞬时值
	Comparator string  `json:"comparator"`  // gt/lt/gte/lte
	Threshold  float64 `json:"threshold"`   // 规则阈值
	Level      string  `json:"level"`       // 建议告警级别（info/warning/critical）
	Source     string  `json:"source"`      // 产生源（mqtt/ocpp）
	OccurredAt int64   `json:"occurred_at"` // 越限时刻（UnixMilli）
}

// consumeCmdFailed 指令失败（FR-OPS-009 系统告警同链路）。
func consumeCmdFailed(ctx context.Context, sc *svc.ServiceContext, env eventbus.Envelope) error {
	var p struct {
		TenantId int64  `json:"tenant_id"`
		DeviceId int64  `json:"device_id"`
		Sn       string `json:"sn"`
		CmdType  string `json:"cmd_type"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.Errorf("ops: cmd_failed payload 不合法 event=%s（丢弃）", env.EventID)
		return nil
	}
	ingestAlert(sc, alertIngest{
		TenantId: envTenant(env, p.TenantId),
		Source:   "CMD_FAILED",
		Metric:   "cmd.failed",
		Level:    "P2",
		Sn:       p.Sn,
		DeviceId: p.DeviceId,
		Title:    "指令执行失败（" + p.Sn + " " + p.CmdType + "）",
		Content:  "指令 " + p.CmdType + " 失败原因：" + p.Reason,
	})
	return nil
}

// consumeOtaPaused OTA 暂停（FR-OPS-009）。
func consumeOtaPaused(ctx context.Context, sc *svc.ServiceContext, env eventbus.Envelope) error {
	var p struct {
		TenantId int64  `json:"tenant_id"`
		TaskId   int64  `json:"task_id"`
		TaskNo   string `json:"task_no"`
		Reason   string `json:"reason"`
		FailPct  int    `json:"fail_pct"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.Errorf("ops: ota_paused payload 不合法 event=%s（丢弃）", env.EventID)
		return nil
	}
	ingestAlert(sc, alertIngest{
		TenantId: envTenant(env, p.TenantId),
		Source:   "OTA_PAUSED",
		Metric:   "ota.paused",
		Level:    "P1",
		Title:    "OTA 任务自动暂停（" + p.TaskNo + "）",
		Content:  "原因：" + p.Reason + "，失败率 " + itoa(p.FailPct) + "%",
	})
	return nil
}

// consumeDeviceAnomaly 设备异常（FR-OPS-009）。
func consumeDeviceAnomaly(ctx context.Context, sc *svc.ServiceContext, env eventbus.Envelope) error {
	var p struct {
		TenantId int64  `json:"tenant_id"`
		DeviceId int64  `json:"device_id"`
		Sn       string `json:"sn"`
		Kind     string `json:"kind"`
		Detail   string `json:"detail"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.Errorf("ops: device_anomaly payload 不合法 event=%s（丢弃）", env.EventID)
		return nil
	}
	kind := p.Kind
	if kind == "" {
		kind = "anomaly"
	}
	ingestAlert(sc, alertIngest{
		TenantId: envTenant(env, p.TenantId),
		Source:   "DEVICE_ANOMALY",
		Metric:   "device." + kind,
		Level:    "P2",
		Sn:       p.Sn,
		DeviceId: p.DeviceId,
		Title:    "设备异常（" + p.Sn + "）",
		Content:  kind + "：" + p.Detail,
	})
	return nil
}

// consumeStationCreated 自动建巡检计划（S7-01，FR-OPS-007）。
func consumeStationCreated(ctx context.Context, sc *svc.ServiceContext, env eventbus.Envelope) error {
	var p struct {
		TenantId    int64  `json:"tenant_id"`
		StationId   int64  `json:"station_id"`
		StationNo   string `json:"station_no"`
		StationName string `json:"name"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.Errorf("ops: station_created payload 不合法 event=%s（丢弃）", env.EventID)
		return nil
	}
	planFromStationCreated(sc, envTenant(env, p.TenantId), p.StationId, p.StationName)
	return nil
}

// ---- 小工具 ----

func envTenant(env eventbus.Envelope, fallback int64) int64 {
	if env.TenantID > 0 {
		return env.TenantID
	}
	return fallback
}

func msTime(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func formatF(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func itoa(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// dedupOnce 消费去重（event_dedup 表；唯一约束幂等；ops-consumer 消费组）。
func dedupOnce(ctx context.Context, sc *svc.ServiceContext, eventID string) (bool, error) {
	var dup bool
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		res, err := session.ExecCtx(ctx,
			"insert ignore into `event_dedup` (`consumer_group`, `event_id`) values (?, ?)",
			"ops-consumer", eventID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		dup = n == 0
		return nil
	})
	return dup, err
}
