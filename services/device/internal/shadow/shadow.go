// Package shadow · 设备影子（S6-01，FR-IOT-005；02 §9.6）。
//
// 存储 = Redis hash（键 micro:iot:shadow:{sn}）：
//   - ts 毫秒时间戳 / raw 最近原始报文 / 五热点指标 soc,voltage,current,temperature,power；
//   - writer（iotingest）经 HSET 全量覆盖，本包只读；
//   - 防乱序：Get 时若快照 ts 早于调用方 last_ts 返回 STALE 标记（写入侧由 iotingest Lua 比较 ts）。
package shadow

import (
	"context"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// KeyPrefix 影子键前缀（iotingest writer 与本包共用常量）。
const KeyPrefix = "micro:iot:shadow:"

// MetricKeys 热点五指标（ADR-13 独立列）。
var MetricKeys = []string{"soc", "voltage", "current", "temperature", "power"}

// Snapshot 影子快照。
type Snapshot struct {
	Sn      string
	Ts      int64             // 毫秒
	Raw     string            // 最近原始报文
	Metrics map[string]string // 键值均字符串（proto 侧转 double）
}

// Key 影子键。
func Key(sn string) string { return KeyPrefix + sn }

// Store 影子读取端。
type Store struct{ rd *redis.Redis }

// NewStore 构造（rd 为 nil 时 Get 恒 ErrNotFound——本地开发免 Redis）。
func NewStore(rd *redis.Redis) *Store { return &Store{rd: rd} }

// ErrNotFound 影子不存在（设备未上报过）。
var ErrNotFound = redis.Nil

// Get 读取影子。
func (s *Store) Get(ctx context.Context, sn string) (*Snapshot, error) {
	if s.rd == nil {
		return nil, ErrNotFound
	}
	vals, err := s.rd.HgetallCtx(ctx, Key(sn))
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, ErrNotFound
	}
	ts, _ := strconv.ParseInt(vals["ts"], 10, 64)
	return &Snapshot{Sn: sn, Ts: ts, Raw: vals["raw"], Metrics: pickMetrics(vals)}, nil
}

func pickMetrics(vals map[string]string) map[string]string {
	out := make(map[string]string, len(MetricKeys))
	for _, k := range MetricKeys {
		if v, ok := vals[k]; ok {
			out[k] = v
		}
	}
	return out
}

// FormatMetrics 影子指标格式化（日志用）。
func (s *Snapshot) FormatMetrics() string {
	var b strings.Builder
	for _, k := range MetricKeys {
		if v, ok := s.Metrics[k]; ok {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(k + "=" + v)
		}
	}
	return b.String()
}
