package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config station RPC 配置（S6-02，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（station_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// ShadowRedis 设备影子只读（Redis DB0，writer=iotingest；GetStationMonitor 聚合用）。
	ShadowRedis redis.RedisConf `json:",optional"`
	// Kafka Outbox relay 投递（station_created）。
	Kafka struct {
		Brokers []string `json:",optional"`
	} `json:",optional"`
	// DeviceRpc 反查设备（GetDeviceStation sn→device_id 归一）。
	DeviceRpc zrpc.RpcClientConf `json:",optional"`
	// MonitorStaleSec 影子在线判定阈值（秒，默认 300s）。
	MonitorStaleSec int `json:",default=300"`
	// ConfigKey configcenter etcd key（ADR-10；键位含文件名）。
	ConfigKey string `json:",optional"`
}
