// Package confcenter · configcenter listener 模板（S3-06 示例；ADR-10，S4 起真实消费）。
//
// 新服务照抄本文件：
//   - 运行时参数（轮询间隔/特性开关/阈值类）走 configcenter（etcd /micro/config/<svc>），
//     启动期配置（端口/DSN）仍走本地 yaml——02 §3.4 两分法；
//   - 正本在 micro-deploy/config/<svc>/（PR 评审），经 tools/configpush 推送 etcd；
//   - AddListener 回调里**只做轻量更新 + 原子替换**，不重载进程、不做阻塞 IO。
package confcenter

import (
	"sync/atomic"

	configurator "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// HelloConf /micro/config/hello 的运行时参数（示例：轮询间隔 + 特性开关）。
type HelloConf struct {
	// PollIntervalMs 前端轮询间隔下推（对齐 admin-bff/polling.yml，S4 起真实生效）。
	PollIntervalMs int `json:"poll_interval_ms"`
	// GreetingFeature 演示特性开关。
	GreetingFeature bool `json:"greeting_feature"`
}

var current atomic.Pointer[HelloConf]

// Current 取当前生效配置（未初始化返回默认值——调用方无需判空）。
func Current() HelloConf {
	if v := current.Load(); v != nil {
		return *v
	}
	return HelloConf{PollIntervalMs: 10000, GreetingFeature: true}
}

// MustListen 订阅 etcd 配置中心（main 启动期调用一次；连不上 etcd 不致命——
// 保留默认值运行，Listener 在网络恢复后自动生效，ADR-10 运行时配置容错口径）。
func MustListen(etcdHosts []string, key string) {
	sub, err := subscriber.NewEtcdSubscriber(subscriber.EtcdConf{
		Hosts: etcdHosts,
		Key:   key,
	})
	if err != nil {
		logx.Errorf("confcenter: etcd subscriber 构造失败（用默认值运行）: %v", err)
		return
	}
	// New 而非 Must：etcd 无初值/不可达不致命——保留默认值运行，
	// Listener 注册后配置首次推送即生效（ADR-10 运行时配置容错口径，S3-06 模板要点）。
	cc, err := configurator.NewConfigCenter[HelloConf](configurator.Config{Type: "yaml"}, sub)
	if err != nil {
		logx.Errorf("confcenter: 订阅构造失败（用默认值运行）: %v", err)
		return
	}

	if v, gerr := cc.GetConfig(); gerr == nil {
		current.Store(&v)
		logx.Infof("confcenter: 初值 poll=%dms feature=%v", v.PollIntervalMs, v.GreetingFeature)
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
		logx.Infof("confcenter: 配置热更新 poll=%dms feature=%v", v.PollIntervalMs, v.GreetingFeature)
	})
}

var _ = redis.MustNewRedis // 预留引用位
