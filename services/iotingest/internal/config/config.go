// Package config · iotingest 配置 Schema（S6-03，02 §9.6 遥测管道）。
package config

import (
	"os"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Config iotingest 独立进程配置（无 RPC 面；OCPP :8182 + metrics :9120）。
type Config struct {
	Name string `json:",default=iotingest"`
	// Prometheus 指标暴露（:9120）。
	Prometheus struct {
		Host string `json:",default=0.0.0.0"`
		Port int    `json:",default=9120"`
		Path string `json:",default=/metrics"`
	} `json:",optional"`
	// Telemetry OTLP 链路（otel，E18）。
	Telemetry struct {
		Name     string  `json:",default=iotingest"`
		Endpoint string  `json:",default=127.0.0.1:24317"`
		Sampler  float64 `json:",default=1.0"`
		Batcher  string  `json:",default=otlpgrpc"`
	} `json:",optional"`
	// Mqtt EMQX 共享订阅（forwarder）。
	Mqtt struct {
		Broker       string `json:",optional"`
		PlatformUser string `json:",optional"`
		PlatformPass string `json:",optional"`
		ClientId     string `json:",default=iotingest-forwarder"`
		// TopicFilter 共享订阅过滤器（02 §9.6：$share/ingest/up/+/+/+/telemetry）。
		TopicFilter string `json:",default=$share/ingest/up/+/+/+/telemetry"`
	} `json:",optional"`
	// Kafka 遥测缓冲（iot_telemetry_raw 进出）。
	Kafka struct {
		Brokers []string `json:",optional"`
		// Group writer 消费组（全局唯一，E2）。
		Group string `json:",default=iotingest-writer"`
		// Topic 遥测原始 topic（下划线，mqinit 预建）。
		Topic string `json:",default=iot_telemetry_raw"`
	} `json:",optional"`
	// Tdengine REST 连接（E7：必须 Basic 认证）。
	Tdengine struct {
		RestUrl string `json:",default=http://127.0.0.1:26041"`
		User    string `json:",default=root"`
		Pass    string `json:",optional"`
		Db      string `json:",default=micro_iot"`
	} `json:",optional"`
	// ShadowRedis 设备影子写入（Redis DB0；键 micro:iot:shadow:{sn}）。
	ShadowRedis redis.RedisConf `json:",optional"`
	// Rules 越限判定（业务判定归 ops；本进程只做候选初筛，FR-IOT-009）。
	Rules struct {
		// TtlSec 规则缓存 TTL（30s，FR-IOT-009 验收口径）。
		TtlSec int64 `json:",default=30"`
		// DefaultRules 兜底规则（configcenter 未推送时生效；告警规则参数兜底值，02 §7.4）。
		DefaultRules []RuleConf `json:",optional"`
	} `json:",optional"`
	// AlertTopic 越限候选 topic。
	AlertTopic string `json:",default=alert_candidate"`
	// ConfigKey configcenter etcd key（ADR-10；键位含文件名）。
	ConfigKey string `json:",optional"`
}

// EtcdHosts etcd 地址（规则订阅用；与 RpcServerConf 无关的独立字段）。
func (c *Config) EtcdHosts() []string {
	return []string{envOr("ETCD_HOSTS", "127.0.0.1:22379")}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// RuleConf 越限规则（metric/comparator/threshold；与 ops alert_rule 字段对齐）。
type RuleConf struct {
	Metric         string  `json:"metric"`     // soc/voltage/current/temperature/power
	Comparator     string  `json:"comparator"` // gt/lt/gte/lte
	Threshold      float64 `json:"threshold"`
	Level          string  `json:",default=warning"` // info/warning/critical
	MinDurationSec int64   `json:",default=0"`       // 持续时长（0=立即；候选初筛不做持续判定，归 ops）
}
