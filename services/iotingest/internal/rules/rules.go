// Package rules · 越限判定规则缓存 + alert_candidate 生产（S6-03，FR-IOT-009）。
//
// 缓存口径（02 §9.6）：collection.Cache LRU（30s TTL）+ configcenter 变更失效 + syncx.SingleFlight
// 防击穿；业务判定归 ops——本进程只做候选初筛（阈值命中 → alert_candidate 事件）。
package rules

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/iotingest/internal/config"
)

func cacheTtl() time.Duration { return 30 * time.Second }

// Evaluator 越限判定器（cache.Take 自带 barrier=SingleFlight 防击穿）。
type Evaluator struct {
	mu       sync.Mutex
	cache    *collection.Cache
	defaults []config.RuleConf
	pusher   Pusher
}

// Pusher alert_candidate 生产抽象。
type Pusher interface {
	PushWithKey(key, val string) error
}

// NewEvaluator 构造（TTL 默认 30s，FR-IOT-009 验收口径）。
func NewEvaluator(defaults []config.RuleConf, ttlSec int64, pusher Pusher) *Evaluator {
	cache, err := collection.NewCache(time.Duration(ttlSec) * time.Millisecond)
	if err != nil {
		logx.Must(err)
	}
	return &Evaluator{cache: cache, defaults: defaults, pusher: pusher}
}

// candidatePayload alert_candidate 载荷（ops 消费判定，FR-OPS-001）。
type candidatePayload struct {
	TenantId   int64   `json:"tenant_id"`
	Sn         string  `json:"sn"`
	ProductKey string  `json:"product_key"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Comparator string  `json:"comparator"`
	Threshold  float64 `json:"threshold"`
	Level      string  `json:"level"`
	Source     string  `json:"source"` // telemetry/ocpp
	OccurredAt int64   `json:"occurred_at"`
}

// Evaluate 单点遥测越限初筛（命中即发 alert_candidate；幂等交给 ops 抑制窗口）。
func (e *Evaluator) Evaluate(ctx context.Context, tenantId int64, pk, sn, payload string, ts int64, source string) {
	rules := e.takeRules(ctx, pk)
	if len(rules) == 0 {
		return
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return
	}
	for _, r := range rules {
		raw, ok := m[r.Metric]
		if !ok {
			continue
		}
		v, ok := raw.(float64)
		if !ok {
			continue
		}
		if hit(r.Comparator, v, r.Threshold) {
			cp := candidatePayload{
				TenantId: tenantId, Sn: sn, ProductKey: pk,
				Metric: r.Metric, Value: v, Comparator: r.Comparator,
				Threshold: r.Threshold, Level: r.Level, Source: source, OccurredAt: ts,
			}
			val, _ := json.Marshal(cp)
			if err := e.pusher.PushWithKey(sn, string(val)); err != nil {
				logx.WithContext(ctx).Errorf("rules: alert_candidate 生产失败 sn=%s metric=%s err=%v", sn, r.Metric, err)
			}
		}
	}
}

// takeRules 取规则集（LRU 30s TTL + Take barrier 防击穿；configcenter 推送由 Reload 失效）。
func (e *Evaluator) takeRules(ctx context.Context, pk string) []config.RuleConf {
	e.mu.Lock()
	cache := e.cache
	e.mu.Unlock()
	rs, err := cache.Take(pk, func() (any, error) {
		// configcenter 快照即最新值（listener 热更新后经 Reload 失效重建）
		cur := confcenterCurrent(pk)
		if len(cur) > 0 {
			return cur, nil
		}
		return e.defaults, nil
	})
	if err != nil {
		return e.defaults
	}
	if out, ok := rs.([]config.RuleConf); ok {
		return out
	}
	return e.defaults
}

// Reload configcenter 变更回调（整体重建缓存，30s 内生效口径，FR-IOT-009）。
func (e *Evaluator) Reload() {
	fresh, err := collection.NewCache(cacheTtl())
	if err == nil {
		e.mu.Lock()
		e.cache = fresh
		e.mu.Unlock()
	}
	logx.Info("rules: configcenter 变更 → 规则缓存已失效（下次评估重载）")
}

func hit(comparator string, v, threshold float64) bool {
	switch comparator {
	case "gt":
		return v > threshold
	case "gte":
		return v >= threshold
	case "lt":
		return v < threshold
	case "lte":
		return v <= threshold
	default:
		return false
	}
}
