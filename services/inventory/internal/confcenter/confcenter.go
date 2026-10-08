// Package confcenter · configcenter listener（S4-05 真实消费，ADR-10）。
//
// 运行时参数（告警兜底值/轮询间隔/开关类）走 configcenter（etcd /micro/config/inventory），
// 启动期配置（端口/DSN）仍走本地 yaml——02 §3.4 两分法。
// 正本在 micro-deploy/config/inventory/（PR 评审），经 tools/configpush 推送 etcd。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// InventoryConf /micro/config/inventory 的运行时参数（S4-05 首批消费：开关/兜底值/间隔）。
type InventoryConf struct {
	// RedisGateEnabled 防超卖 Redis 预扣网关运行时开关（false = 纯 DB 路径，仍不超卖）。
	RedisGateEnabled bool `json:"redis_gate_enabled"`
	// ReconcileIntervalSec Redis↔DB 对账周期（cron inventory-redis-reconcile 消费）。
	ReconcileIntervalSec int `json:"reconcile_interval_sec"`
	// LowStockAlertFallbackMs 低库存告警兜底节流（ms）——告警兜底值首批消费项。
	LowStockAlertFallbackMs int `json:"low_stock_alert_fallback_ms"`
}

var current atomic.Pointer[InventoryConf]

// Current 取当前生效配置（未初始化返回默认值——调用方无需判空）。
func Current() InventoryConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return InventoryConf{RedisGateEnabled: true, ReconcileIntervalSec: 30, LowStockAlertFallbackMs: 60000}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[InventoryConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 gate=%v reconcile=%ds alert_fallback=%dms", v.RedisGateEnabled, v.ReconcileIntervalSec, v.LowStockAlertFallbackMs)
					} else {
						logx.Infof("confcenter: etcd 尚无初值（用默认值，推送后自动生效）")
					}
					cc.AddListener(func() {
						v, err := cc.GetConfig()
						if err != nil {
							logx.Errorf("confcenter: 拉取新配置失败（保留旧值）: %v", err)
							return
						}
						current.Store(&v)
						logx.Infof("confcenter: 配置热更新 gate=%v reconcile=%ds alert_fallback=%dms", v.RedisGateEnabled, v.ReconcileIntervalSec, v.LowStockAlertFallbackMs)
					})
					return
				}
				logx.Errorf("confcenter: 订阅构造失败（30s 后重试）: %v", cerr)
			} else {
				logx.Errorf("confcenter: etcd subscriber 构造失败（30s 后重试）: %v", err)
			}
			time.Sleep(30 * time.Second)
		}
	}()
}
