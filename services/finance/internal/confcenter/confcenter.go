// Package confcenter · configcenter listener（S5-03，ADR-10）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// FinanceConf /micro/config/finance 的运行时参数。
type FinanceConf struct {
	// MockGateway dev 模拟网关（冒烟不依赖真实商户号；生产必须 false——Git 正本把关）。
	MockGateway bool `json:"mock_gateway"`
	// ChannelDegraded 渠道降级开关（02 §10：微信/支付宝渠道故障时只收对公）。
	ChannelDegraded bool `json:"channel_degraded"`
	// WechatEnabled 微信渠道开关。
	WechatEnabled bool `json:"wechat_enabled"`
	// AlipayEnabled 支付宝渠道开关。
	AlipayEnabled bool `json:"alipay_enabled"`
}

var current atomic.Pointer[FinanceConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() FinanceConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return FinanceConf{MockGateway: true, ChannelDegraded: false, WechatEnabled: true, AlipayEnabled: true}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[FinanceConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 mock=%v degraded=%v wechat=%v alipay=%v",
							v.MockGateway, v.ChannelDegraded, v.WechatEnabled, v.AlipayEnabled)
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
						logx.Infof("confcenter: 配置热更新 mock=%v degraded=%v wechat=%v alipay=%v",
							v.MockGateway, v.ChannelDegraded, v.WechatEnabled, v.AlipayEnabled)
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
