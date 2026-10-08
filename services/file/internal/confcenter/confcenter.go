// Package confcenter · configcenter listener（S4-05，ADR-10）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// FileConf /micro/config/file 的运行时参数。
type FileConf struct {
	// TokenExpSec 下载令牌有效期兜底值（confcenter 首批消费：告警兜底值类）。
	TokenExpSec int `json:"token_exp_sec"`
	// UploadEnabled 上传开关（运维可一键停写）。
	UploadEnabled bool `json:"upload_enabled"`
}

var current atomic.Pointer[FileConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() FileConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return FileConf{TokenExpSec: 300, UploadEnabled: true}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[FileConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 token_exp=%ds upload=%v", v.TokenExpSec, v.UploadEnabled)
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
						logx.Infof("confcenter: 配置热更新 token_exp=%ds upload=%v", v.TokenExpSec, v.UploadEnabled)
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
