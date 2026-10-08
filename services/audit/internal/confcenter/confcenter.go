// Package confcenter · configcenter listener（S4-05，ADR-10）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// AuditConf /micro/config/audit 的运行时参数。
type AuditConf struct {
	// WriteEnabled 审计写入开关（应急熔断：审计风暴时保护主库，恢复后自动追平由事件侧保证）。
	WriteEnabled bool `json:"write_enabled"`
	// RetainYears 保留年限（验收口径 ≥3 年，S11 归档策略对齐）。
	RetainYears int `json:"retain_years"`
}

var current atomic.Pointer[AuditConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() AuditConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return AuditConf{WriteEnabled: true, RetainYears: 3}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[AuditConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 write=%v retain=%dy", v.WriteEnabled, v.RetainYears)
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
						logx.Infof("confcenter: 配置热更新 write=%v retain=%dy", v.WriteEnabled, v.RetainYears)
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
