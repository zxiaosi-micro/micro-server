// Package confcenter · configcenter listener（S8，ADR-10；30s 重试版模板，照抄 ops）。
package confcenter

import (
	"sync/atomic"
	"time"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
)

// AnalyticsConf /micro/config/analytics/main 的运行时参数。
type AnalyticsConf struct {
	ExportWorkers  int    `json:"export_workers"`   // 导出 TaskRunner 并发（0=用启动配置）
	ExportPdfRows  int    `json:"export_pdf_rows"`  // PDF 单文件行数上限（0=用启动配置）
	ReconEnabled   bool   `json:"recon_enabled"`    // 日级对账 cron 开关（false=不跑）
	ReconTargetUID int64  `json:"recon_target_uid"` // 差异站内信收件人（0=平台管理员 seed uid）
	PdfFont        string `json:"pdf_font"`         // PDF 中文字体（ttf 路径；空=用启动配置）
}

var current atomic.Pointer[AnalyticsConf]

// Current 取当前生效配置（未初始化返回默认值）。
func Current() AnalyticsConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return AnalyticsConf{}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次）。
// 容错口径（ADR-10 + E15）：etcd 不可达/尚无初值不致命——保留默认值运行；
// 后台每 30s 重试订阅，首次推送或网络恢复后自动接入并热更新。
func MustListen(etcdHosts []string, key string) {
	go func() {
		for {
			sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{Hosts: etcdHosts, Key: key})
			if err == nil {
				cc, cerr := configurator.NewConfigCenter[AnalyticsConf](configurator.Config{Type: "yaml"}, sub)
				if cerr == nil {
					if v, gerr := cc.GetConfig(); gerr == nil {
						current.Store(&v)
						logx.Infof("confcenter: 初值 export_workers=%d", v.ExportWorkers)
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
						logx.Infof("confcenter: 配置热更新 export_workers=%d recon_enabled=%v", v.ExportWorkers, v.ReconEnabled)
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
