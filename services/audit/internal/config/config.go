package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config audit RPC 配置（02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
