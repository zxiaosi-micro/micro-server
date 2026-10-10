// Package confcenter · configcenter listener（S6-03，ADR-10；30s 重试版）。
// 规则推送键 = /micro/config/iotingest/rules（与启动配置 /micro/config/iotingest/main 分离，
// 规则变更回调失效 LRU 缓存——FR-IOT-009 30s 生效口径）。
package confcenter

import (
	"sync"
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/iotingest/internal/config"
)

// RulesConf /micro/config/iotingest/rules 的规则集。
type RulesConf struct {
	Rules []config.RuleConf `json:"rules"`
}

var (
	current   atomic.Pointer[RulesConf]
	onReloads []func()
	reloadMu  sync.Mutex
)

// Current 取当前规则快照（未推送返回 nil——调用方用默认规则）。
func Current() *RulesConf {
	if v := current.Load(); v != nil {
		return v
	}
	return nil
}

// OnReload 注册变更回调（LRU 失效用）。
func OnReload(fn func()) {
	reloadMu.Lock()
	defer reloadMu.Unlock()
	onReloads = append(onReloads, fn)
}

// MustListen 订阅规则配置（main 启动期调用；30s 重试接入，E15 容错口径）。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[RulesConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 规则初值 n=%d", len(v.Rules))
					} else {
						logx.Info("confcenter: 规则键尚无初值（用启动默认规则，推送后生效）")
					}
					cc.AddListener(func() {
						v, err := cc.GetConfig()
						if err != nil {
							logx.Errorf("confcenter: 拉取新规则失败（保留旧值）: %v", err)
							return
						}
						current.Store(&v)
						logx.Infof("confcenter: 规则热更新 n=%d", len(v.Rules))
						reloadMu.Lock()
						fns := append([]func(){}, onReloads...)
						reloadMu.Unlock()
						for _, fn := range fns {
							fn()
						}
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
