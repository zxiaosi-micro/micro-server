package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config contract RPC 配置（S5-04，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（contract_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Kafka 事件消费（device_activated/shipment_signed → 质保起算）+ Outbox relay。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group 消费组（全局唯一，E2）。
		Group string `json:",default=contract-warranty"`
	} `json:",optional"`
	// FileRpc file 服务客户端（归档件受控下载留位；本体存合同桶）。
	FileRpc zrpc.RpcClientConf `json:",optional"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
