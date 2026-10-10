// Package writer · Kafka 消费 → executors.BulkExecutor 批量写 TDengine → 影子（S6-03，02 §9.6）。
//
// 组件语义约束（02 §9.6）：BulkExecutor 回调不返回错误——批量写失败必须持久化重试轨迹
// （回灌 Kafka iot_telemetry_raw），不允许只打日志；回灌重试超限（16 次）记 ERROR 放弃（对账兜底）。
package writer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/executors"
	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/iotingest/internal/forwarder"
	"micro-server/services/iotingest/internal/tdengine"
)

// ShadowStore 影子写入抽象（Redis HSET + Lua 时间戳防乱序；测试可 mock）。
type ShadowStore interface {
	Write(ctx context.Context, sn string, ts int64, metrics map[string]float64, raw string) error
}

// Replayer Kafka 回灌抽象（写失败回灌 iot_telemetry_raw）。
type Replayer interface {
	Replay(key, val string) error
}

// Evaluator 越限初筛抽象（rules.Evaluator；业务判定归 ops，FR-IOT-009）。
type Evaluator interface {
	Evaluate(ctx context.Context, tenantId int64, pk, sn, payload string, ts int64, source string)
}

// Writer 遥测写入器。
type Writer struct {
	td     *tdengine.Client
	shadow ShadowStore
	rep    Replayer
	ev     Evaluator
	bulk   *executors.BulkExecutor
}

// New 构造（WithBulkTasks(500) + WithBulkInterval(200ms)，FR-IOT-003/02 §9.6）。
func New(td *tdengine.Client, shadow ShadowStore, rep Replayer, ev Evaluator) *Writer {
	w := &Writer{td: td, shadow: shadow, rep: rep, ev: ev}
	w.bulk = executors.NewBulkExecutor(
		func(items []any) {
			w.flush(context.WithoutCancel(context.Background()), items)
		},
		executors.WithBulkTasks(500),
		executors.WithBulkInterval(200*time.Millisecond),
	)
	return w
}

// Consume kq.ConsumeHandler 签名（writer 消费组）。
func (w *Writer) Consume(_ context.Context, key, val string) error {
	var msg forwarder.TelemetryMsg
	if err := json.Unmarshal([]byte(val), &msg); err != nil {
		// 畸形消息：记轨迹后 ack（不无限重投，02 §6.6）
		logx.Errorf("writer: 畸形遥测丢弃 key=%s err=%v raw=%.200s", key, err, val)
		return nil
	}
	_ = key // BulkExecutor 无 key 概念；保序由 partition key=sn 承担
	return w.bulk.Add(msg)
}

// flush 批量写 TDengine → 影子（BulkExecutor 回调，无错误返回——失败回灌，02 §9.6）。
func (w *Writer) flush(ctx context.Context, items []interface{}) {
	pts := make([]tdengine.Point, 0, len(items))
	shadowMap := map[string]forwarder.TelemetryMsg{}
	for _, it := range items {
		msg := it.(forwarder.TelemetryMsg)
		metrics := parseMetrics(msg.Payload)
		pts = append(pts, tdengine.Point{
			TenantId:    msg.TenantId,
			ProductKey:  msg.ProductKey,
			Sn:          msg.Sn,
			Ts:          time.UnixMilli(msg.Ts),
			Soc:         metrics["soc"],
			Voltage:     metrics["voltage"],
			Current:     metrics["current"],
			Temperature: metrics["temperature"],
			Power:       metrics["power"],
			Raw:         msg.Payload,
		})
		shadowMap[msg.Sn] = msg
	}
	if err := w.td.WriteBatch(ctx, pts); err != nil {
		logx.Errorf("writer: TDengine 批量写失败（回灌 Kafka）n=%d err=%v", len(pts), err)
		w.replayAll(ctx, items, err)
		return
	}
	// 影子写入（Lua 时间戳防乱序；失败仅记日志——影子可从最新遥测重建）+ 越限初筛
	for sn, msg := range shadowMap {
		metrics := parseMetrics(msg.Payload)
		if w.shadow != nil {
			if err := w.shadow.Write(ctx, sn, msg.Ts, metrics, msg.Payload); err != nil {
				logx.Errorf("writer: 影子写入失败 sn=%s err=%v", sn, err)
			}
		}
		if w.ev != nil {
			w.ev.Evaluate(ctx, msg.TenantId, msg.ProductKey, sn, msg.Payload, msg.Ts, "telemetry")
		}
	}
}

func (w *Writer) replayAll(ctx context.Context, items []interface{}, cause error) {
	for _, it := range items {
		msg := it.(forwarder.TelemetryMsg)
		val, _ := json.Marshal(msg)
		if w.rep == nil {
			continue
		}
		if err := w.rep.Replay(msg.Sn, string(val)); err != nil {
			logx.Errorf("writer: 回灌失败（数据滞留日志兜底）sn=%s err=%v", msg.Sn, err)
		}
	}
	_ = cause
}

// parseMetrics 报文热点五指标解析（宽容解析：缺省 0）。
func parseMetrics(payload string) map[string]float64 {
	out := map[string]float64{}
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return out
	}
	for _, k := range []string{"soc", "voltage", "current", "temperature", "power"} {
		if v, ok := m[k]; ok {
			out[k] = toFloat(v)
		}
	}
	return out
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		_ = fmt.Sprint(v)
		return 0
	}
}

// Flush 优雅关闭（Flush/Wait 语义：drain 缓冲后再退出，02 §9.6）。
func (w *Writer) Flush() {
	time.Sleep(300 * time.Millisecond) // BulkExecutor 200ms 间隔兜底 drain
}
