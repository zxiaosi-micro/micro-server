// Package confcenter · configcenter listener（S5-02，ADR-10）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// OrderConf /micro/config/order 的运行时参数。
type OrderConf struct {
	// PayTimeoutMinutes 支付超时分钟数（0=用启动配置；热调用于大促宽限期场景）。
	PayTimeoutMinutes int `json:"pay_timeout_minutes"`
	// AdvanceBatch 扫描推进批量（Saga 重试/超时扫描单批条数）。
	AdvanceBatch int `json:"advance_batch"`
}

var current atomic.Pointer[OrderConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() OrderConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return OrderConf{PayTimeoutMinutes: 0, AdvanceBatch: 100}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[OrderConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 pay_timeout=%d advance_batch=%d", v.PayTimeoutMinutes, v.AdvanceBatch)
					} else {
						logx.Info("confcenter: etcd 尚无初值（用默认值，推送后自动生效）")
					}
					cc.AddListener(func() {
						v, err := cc.GetConfig()
						if err != nil {
							logx.Errorf("confcenter: 拉取新配置失败（保留旧值）: %v", err)
							return
						}
						current.Store(&v)
						logx.Infof("confcenter: 配置热更新 pay_timeout=%d advance_batch=%d", v.PayTimeoutMinutes, v.AdvanceBatch)
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
