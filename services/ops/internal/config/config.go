package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config ops RPC 配置（S7-01，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（ops_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Kafka 事件消费（alert_candidate 等 5 topic）+ Outbox relay（alert_created）。
	Kafka struct {
		Brokers []string `json:",optional"`
		Group   string   `json:",optional"`
	} `json:",optional"`
	// StationRpc 设备→场站归属（GetDeviceStation sn→station）。
	StationRpc zrpc.RpcClientConf `json:",optional"`
	// PartyRpc 派单候选（ListStaff 区域+技能匹配）。
	PartyRpc zrpc.RpcClientConf `json:",optional"`
	// ContractRpc SLA 策略正本（ListSlaStrategy 透传 + 快照冻结）。
	ContractRpc zrpc.RpcClientConf `json:",optional"`
	// AcceptTimeoutMin 接单超时（分钟，DispatchTicket 默认；confcenter 热调）。
	AcceptTimeoutMin int `json:",default=30"`
	// ConfigKey configcenter etcd key（ADR-10；键位含文件名）。
	ConfigKey string `json:",optional"`
}
