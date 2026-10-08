package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config file RPC 配置（02 §3.4）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// MinIO S3 兼容对象存储（02 §6.5；双桶 micro-file/micro-contract）。
	Minio struct {
		Endpoint       string `json:",default=127.0.0.1:29000"`
		AccessKey      string `json:",optional"`
		SecretKey      string `json:",optional"`
		BucketDefault  string `json:",default=micro-file"`
		BucketContract string `json:",default=micro-contract"`
		UseSSL         bool   `json:",default=false"`
	} `json:",optional"`
	// SignKeyEnv 下载令牌 HMAC 密钥环境变量名。
	SignKeyEnv string `json:",default=MICRO_FILE_SIGN_KEY"`
	// DefaultTokenExpSec 临时下载令牌缺省有效期。
	DefaultTokenExpSec int `json:",default=300"`
	// ConfigKey configcenter etcd key（ADR-10）。
	ConfigKey string `json:",optional"`
}
