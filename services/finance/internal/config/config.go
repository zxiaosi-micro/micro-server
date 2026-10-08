package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config finance RPC 配置（S5-03，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（finance_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Kafka 事件消费（order_return_approved → 原路退回）+ Outbox relay。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group 消费组（全局唯一，E2）。
		Group string `json:",default=finance-refund"`
	} `json:",optional"`
	// Wechat 微信支付 v3（JSAPI；密钥走 env 注入，仓库零真实密钥，E11）。
	Wechat struct {
		MchID        string `json:",optional"`
		MchSerialNo  string `json:",optional"`
		AppID        string `json:",optional"`
		MchPrivateKey string `json:",optional"` // 商户私钥 PEM（env 注入）
		PlatPublicKey string `json:",optional"` // 微信平台公钥 PEM（回调验签）
		APIv3Key     string `json:",optional"` // 回调资源 AES-256-GCM 解密
	} `json:",optional"`
	// Alipay 支付宝（RSA2）。
	Alipay struct {
		AppID        string `json:",optional"`
		AppPrivateKey string `json:",optional"` // 应用私钥 PEM
		AlipayPublicKey string `json:",optional"` // 支付宝公钥 PEM（回调验签）
		Gateway      string `json:",default=https://openapi.alipay.com/gateway.do"`
	} `json:",optional"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
