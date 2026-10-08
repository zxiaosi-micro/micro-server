package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config inventory RPC 配置（02 §3.4：结构体是唯一契约，敏感值 ${ENV} 注入）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存；段首节点同时作为防超卖 Redis 预扣网关。
	Cache cache.CacheConf `json:",optional"`
	// RedisGate 防超卖 Redis 预扣网关（02 §9.2 第一层；故障自动降级纯 DB）。
	RedisGate struct {
		// Enabled 启动期总开关（运行期由 configcenter redis_gate_enabled 热切换）。
		Enabled bool `json:",default=true"`
	} `json:",optional"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
