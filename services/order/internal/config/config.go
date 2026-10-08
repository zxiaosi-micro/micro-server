package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config order RPC 配置（S5-02，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（order_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Kafka outbox relay 投递 + 事件消费（order_paid/stock_out/payment_refunded）。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group 消费组（全局唯一，E2；event_dedup 键前缀）。
		Group string `json:",default=order-saga"`
	} `json:",optional"`
	// 下游 RPC（Saga 步骤动作触达；韧性按 02 §10：内部 RPC 3~5s 超时，禁重试——Saga/事件兜底）。
	InventoryRpc zrpc.RpcClientConf `json:",optional"`
	FinanceRpc   zrpc.RpcClientConf `json:",optional"`
	ContractRpc  zrpc.RpcClientConf `json:",optional"`
	CatalogRpc   zrpc.RpcClientConf `json:",optional"`
	// PayTimeoutMinutes 支付超时分钟数（FR-ORD-006：30 分钟自动取消，pay_expire_at 扫描）。
	PayTimeoutMinutes int `json:",default=30"`
	// SagaRetryBackoffMinutes Saga 步骤失败重试退避序列（分钟，02 §9.1：1m/5m/30m）。
	SagaRetryBackoffMinutes []int `json:",default=[1,5,30]"`
	// SagaMaxRetry 当前步骤最大重试次数，超限 → 人工介入队列（02 §9.1）。
	SagaMaxRetry int `json:",default=3"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
