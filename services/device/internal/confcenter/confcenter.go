// Package confcenter · configcenter listener（S6-01，ADR-10；30s 重试版，照抄 order 模板）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// DeviceConf /micro/config/device/main 的运行时参数。
type DeviceConf struct {
	CmdAckTimeoutMs    int64 `json:"cmd_ack_timeout_ms"`    // 0=用服务启动配置（3s）
	CmdRetryBackoffSec int   `json:"cmd_retry_backoff_sec"` // 0=用服务启动配置（60s）
	OtaDispatchBatch   int   `json:"ota_dispatch_batch"`    // OTA 单轮下发上限（0=用任务 batch_size）
	ProvisionEnabled   bool  `json:"provision_enabled"`     // EMQX 凭证开通开关（EMQX 运维窗口可临时关闭）
}

var current atomic.Pointer[DeviceConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() DeviceConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return DeviceConf{}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[DeviceConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 cmd_ack=%dms ota_batch=%d", v.CmdAckTimeoutMs, v.OtaDispatchBatch)
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
						logx.Infof("confcenter: 配置热更新 cmd_ack=%dms ota_batch=%d provision=%v",
							v.CmdAckTimeoutMs, v.OtaDispatchBatch, v.ProvisionEnabled)
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
