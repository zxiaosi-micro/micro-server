// 告警内核：候选/系统事件 → 判定（抑制窗口/级别）→ 场站聚合 → 入库 +
// alert_created 事件 + notification 持久通知（FR-OPS-001/002/009，01 §4.3 链路）。
package logic

import (
	"context"
	"encoding/json"
	"time"

	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/internal/svc"

	stationclient "micro-server/services/station/station"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// alertIngest 统一告警入口载荷（事件消费侧归一后传入）。
type alertIngest struct {
	TenantId   int64
	Source     string    // TELEMETRY/CMD_FAILED/OTA_PAUSED/DEVICE_ANOMALY
	Metric     string    // 指标名或系统事件名（cmd.failed/ota.paused/device.anomaly）
	Comparator string    // 越限比较符（遥测类才有，如 gt/lt）
	Threshold  *float64  // 规则阈值（遥测类才有）
	LastValue  *float64  // 越限瞬时值（遥测类才有）
	Level      string    // P1/P2/P3
	Sn         string    // 设备 SN（归属聚合键；系统告警可空）
	DeviceId   int64     // 设备 ID（SN 反查失败时兜底携带）
	Title      string    // 告警标题
	Content    string    // 告警详情
	OccurredAt time.Time // 发生时刻（零值取当前时间）
}

// ingestAlert 告警判定与入库（幂等口径：抑制窗口内同聚合键累计，不重复开告警）。
// 事务范围 = 告警行写入/聚合更新 + alert_created Outbox（同事务原子）；
// 通知走独立 Outbox（允许最终一致）。
func ingestAlert(sc *svc.ServiceContext, in alertIngest) {
	ctx := tenantx.WithTenant(context.Background(), in.TenantId)
	now := in.OccurredAt
	if now.IsZero() {
		now = time.Now()
	}

	// 1) 归属聚合：sn → 场站（station RPC 缺失/未绑定 = 平台级 station_id=0）
	stationId, stationName := resolveStation(sc, in.Sn, in.DeviceId)

	// 2) 命中规则取抑制窗口（规则未配置用默认 300s）
	suppressSec := int64(300)
	if rules, err := sc.Models.AlertRule.ListEnabledByMetric(ctx, in.TenantId, in.Metric); err == nil && len(rules) > 0 {
		suppressSec = rules[0].SuppressWindowSec
		in.Level = rules[0].Level // 规则级别优先于候选自带（FR-OPS-001 正本口径）
	}
	if in.Level == "" {
		in.Level = "P2"
	}

	// 3) 聚合判定：抑制窗口内同聚合键（tenant+station+source+metric+level）活跃告警
	var agg *model.Alert
	windowStart := now.Add(-time.Duration(suppressSec) * time.Second)
	if a, err := sc.Models.Alert.FindActiveForAgg(ctx, in.TenantId, stationId, in.Source, in.Metric, in.Level, windowStart); err == nil {
		agg = a
	} else if err != model.ErrNotFound {
		logx.Errorf("ops: 聚合查找失败 tenant=%d metric=%s: %v", in.TenantId, in.Metric, err)
		return
	}

	devices := appendDeviceJSON("", in.Sn, in.DeviceId)
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if agg != nil {
			// 抑制窗口内：聚合累计（FR-OPS-002 同站多设备合并）
			var lv *float64
			if in.LastValue != nil {
				lv = in.LastValue
			}
			return sc.Models.Alert.AggHitTx(ctx, session, agg.AlertId, lv, devices, now)
		}
		// 新告警
		row := &model.Alert{
			AlertId:        sc.Snowflake.MustNextID(),
			Level:          in.Level,
			Source:         in.Source,
			Metric:         in.Metric,
			Comparator:     in.Comparator,
			StationId:      stationId,
			StationName:    stationName,
			DeviceId:       in.DeviceId,
			Sn:             in.Sn,
			Title:          in.Title,
			Content:        in.Content,
			Status:         "OPEN",
			AggregateCount: 1,
			Devices:        sqlNullString(devices),
			FirstSeenAt:    now,
			LastSeenAt:     now,
			TenantId:       in.TenantId,
		}
		if in.Threshold != nil {
			row.Threshold = nullFloat(*in.Threshold)
		}
		if in.LastValue != nil {
			row.LastValue = nullFloat(*in.LastValue)
		}
		if err := sc.Models.Alert.InsertTx(ctx, session, row); err != nil {
			return err
		}
		// alert_created 事件（Outbox 同事务；分析域/订阅消息场景广播）
		// Payload 必须传 map（known-pit：预序列化 []byte 会被信封 base64 编码，消费方解析失败）
		return eventbus.Emit(ctx, session, eventbus.EmitInput{
			Topic:    eventbus.TopicAlertCreated,
			Type:     eventbus.TypeAlertCreated,
			Key:      in.Sn,
			TenantID: in.TenantId,
			Payload: map[string]any{
				"alert_id": row.AlertId, "tenant_id": in.TenantId,
				"station_id": stationId, "level": in.Level, "source": in.Source,
				"metric": in.Metric, "title": in.Title, "first_seen_at": now.UnixMilli(),
			},
		})
	})
	if err != nil {
		logx.Errorf("ops: 告警入库失败 tenant=%d metric=%s sn=%s: %v", in.TenantId, in.Metric, in.Sn, err)
		return
	}

	if agg == nil {
		bumpAlertUnread(sc, in.TenantId, stationId)
		notifyStationStaff(sc, in.TenantId, stationId, in.Level, in.Title)
	}
	logx.Infof("ops: 告警%s tenant=%d station=%d metric=%s level=%s",
		map[bool]string{true: "聚合", false: "新建"}[agg != nil], in.TenantId, stationId, in.Metric, in.Level)
}

// resolveStation sn/device → 场站（失败降级平台级，不阻塞告警链）。
func resolveStation(sc *svc.ServiceContext, sn string, deviceId int64) (int64, string) {
	if sc.Station == nil || (sn == "" && deviceId == 0) {
		return 0, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	resp, err := sc.Station.GetDeviceStation(ctx, &stationclient.GetDeviceStationReq{
		Sn: sn, DeviceId: deviceId,
	})
	if err != nil || resp == nil || resp.Station == nil {
		return 0, ""
	}
	return resp.Station.StationId, resp.Station.StationName
}

// notifyStationStaff 场站人员持久通知（无驻场人员 = 不发，轮询兜底可见）。
func notifyStationStaff(sc *svc.ServiceContext, tenantId, stationId int64, level, title string) {
	if stationId == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	resp, err := sc.Station.ListStationStaff(ctx, &stationclient.ListStationStaffReq{StationId: stationId})
	if err != nil || resp == nil {
		return
	}
	for _, s := range resp.List {
		notify(sc, tenantId, s.UserId, "alert_created", "ALERT",
			"alert:"+title, map[string]string{"level": level, "title": title})
	}
}

// appendDeviceJSON 聚合设备明细追加（[{"sn":"","device_id":0}]）。
func appendDeviceJSON(cur, sn string, deviceId int64) string {
	type dev struct {
		Sn       string `json:"sn,omitempty"`
		DeviceId int64  `json:"device_id,omitempty"`
	}
	var list []dev
	if cur != "" {
		_ = json.Unmarshal([]byte(cur), &list)
	}
	if sn != "" || deviceId > 0 {
		list = append(list, dev{Sn: sn, DeviceId: deviceId})
	}
	b, _ := json.Marshal(list)
	return string(b)
}

// ctxkit 引用占位（消费侧租户恢复由 consumer 包完成，此处仅防误删 import）。
var _ = ctxkit.UID
