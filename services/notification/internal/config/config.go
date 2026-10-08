package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config notification RPC 配置（02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Kafka notification_request 事件消费（FR-NTF-001：业务只发事件不直连渠道）。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group 消费组（event_dedup 键前缀）。
		Group string `json:",default=notification-deliver"`
	} `json:",optional"`
	// RateLimitPer24h 频控：同用户同模板 24h 内上限（FR-NTF-002，confcenter 可热调）。
	RateLimitPer24h int `json:",default=3"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
