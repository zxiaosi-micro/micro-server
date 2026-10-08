package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config party RPC 配置（02 §3.4：结构体是唯一契约，敏感值 ${ENV} 注入）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存（go-zero sqlc）。
	Cache cache.CacheConf `json:",optional"`
	// DataKeysEnv 数据加密密钥环境变量名（MICRO_DATA_KEYS，contact.mobile 加密）。
	DataKeysEnv string `json:",default=MICRO_DATA_KEYS"`
	// DataKeyKIDEnv 当前加密 kid 环境变量名。
	DataKeyKIDEnv string `json:",default=MICRO_DATA_KEY_KID"`
	// HashKeyEnv HMAC-SHA256 索引键环境变量名（contact.mobile_hash）。
	HashKeyEnv string `json:",default=MICRO_HASH_KEY"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
