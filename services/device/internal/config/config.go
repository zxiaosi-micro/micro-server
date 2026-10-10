package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config device RPC 配置（S6-01，02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源（device_db）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// ShadowRedis 设备影子存储（Redis DB0，键 micro:iot:shadow:{sn}；writer=iotingest）。
	ShadowRedis redis.RedisConf `json:",optional"`
	// Kafka Outbox relay 投递 + 事件消费（stock_in/stock_out/cmd_ack）。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group 消费组（全局唯一，E2；device-asset）。
		Group string `json:",default=device-asset"`
	} `json:",optional"`
	// Emqx MQTT 链路（指令下行发布 + 凭证开通 REST）。
	Emqx struct {
		// Broker 指令下行 MQTT 地址（dev: tcp://127.0.0.1:21883）。
		Broker string `json:",optional"`
		// PlatformUser/Pass 平台后端账号（ACL：发布 down/#、订阅 up/#；bootstrap.sh 建）。
		PlatformUser string `json:",optional"`
		PlatformPass string `json:",optional"`
		// ApiBase Dashboard REST（dev: http://127.0.0.1:38083；凭证开通写一机一密）。
		ApiBase       string `json:",optional"`
		DashboardUser string `json:",optional"`
		DashboardPass string `json:",optional"`
	} `json:",optional"`
	// OtaPublicKeyPath Ed25519 公钥（PEM；keygen 产出 ota_ed25519_public.pem，FR-IOT-007）。
	OtaPublicKeyPath string `json:",optional"`
	// AuditRpc 指令独立审计（audit.WriteCmdLog；未配置跳过——cmd 表留痕兜底）。
	AuditRpc zrpc.RpcClientConf `json:",optional"`
	// CmdAckTimeoutMs TimingWheel ACK 超时（FR-IOT-006 3s）。
	CmdAckTimeoutMs int64 `json:",default=3000"`
	// CmdRetryBackoffSec 指令重试退避秒数（cron 兜底扫描）。
	CmdRetryBackoffSec int `json:",default=60"`
	// OtaDispatchIntervalSec OTA 批次下发扫描间隔（cron）。
	OtaDispatchIntervalSec int `json:",default=30"`
	// ConfigKey configcenter etcd key（ADR-10；键位含文件名）。
	ConfigKey string `json:",optional"`
}
