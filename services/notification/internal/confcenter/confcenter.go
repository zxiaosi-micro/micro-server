// Package confcenter · configcenter listener（S4-05，ADR-10）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// NotificationConf /micro/config/notification 的运行时参数。
type NotificationConf struct {
	// RateLimitPer24h 频控上限（同用户同模板 24h 内，FR-NTF-002）。
	RateLimitPer24h int `json:"rate_limit_per_24h"`
	// DeliverEnabled 投递总开关（事件消费与 RPC Deliver 共用）。
	DeliverEnabled bool `json:"deliver_enabled"`
}

var current atomic.Pointer[NotificationConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() NotificationConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return NotificationConf{RateLimitPer24h: 3, DeliverEnabled: true}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[NotificationConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 rate=%d/24h deliver=%v", v.RateLimitPer24h, v.DeliverEnabled)
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
						logx.Infof("confcenter: 配置热更新 rate=%d/24h deliver=%v", v.RateLimitPer24h, v.DeliverEnabled)
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
