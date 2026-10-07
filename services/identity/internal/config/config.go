package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config identity RPC 配置（02 §3.4：结构体是唯一契约，敏感值 ${ENV} 注入）。
type Config struct {
	zrpc.RpcServerConf
	// Mysql 数据源。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存（go-zero sqlc）。
	Cache cache.CacheConf `json:",optional"`
	// Sessionx 会话中心（Redis DB4，02 §6.3）。
	Sessionx struct {
		Addr       string        `json:",default=127.0.0.1:26379"`
		Password   string        `json:",optional"`
		DB         int           `json:",default=4"`
		SessionTTL time.Duration `json:",default=604800s"`
		RefreshTTL time.Duration `json:",default=604800s"`
		StepUpTTL  time.Duration `json:",default=300s"`
	} `json:",optional"`
	// Jwt RS256 签发密钥（keygen 产出；KeysDir 下 keys.json + PEM）。
	Jwt struct {
		// KeysDir 密钥目录（deploy/conf/keys），优先级低于显式 PrivateFile/PublicKeyFile。
		KeysDir string `json:",optional"`
		// PrivateFile/PublicKeyFile 显式 PEM 路径（单钥模式，测试用）。
		PrivateKeyFile string        `json:",optional"`
		PublicKeyFile  string        `json:",optional"`
		AccessTTL      time.Duration `json:",default=1800s"`
	} `json:",optional"`
	// DataKeysEnv 数据加密密钥环境变量名（MICRO_DATA_KEYS，keygen 产出注入 env）。
	DataKeysEnv string `json:",default=MICRO_DATA_KEYS"`
	// DataKeyKIDEnv 当前加密 kid 环境变量名。
	DataKeyKIDEnv string `json:",default=MICRO_DATA_KEY_KID"`
	// HashKeyEnv HMAC-SHA256 索引键环境变量名（mobile_hash/email_hash）。
	HashKeyEnv string `json:",default=MICRO_HASH_KEY"`
	// Lockout 登录失败锁定策略（02 §9.5：5 次失败锁 30 分钟）。
	Lockout struct {
		MaxFails   int           `json:",default=5"`
		FailWindow time.Duration `json:",default=900s"` // 15min 计数窗口
		LockDur    time.Duration `json:",default=1800s"`
	} `json:",optional"`
	// Wechat 微信 code2session（dev 可不配，LoginByWechat 返回未实现错误）。
	Wechat struct {
		AppID     string `json:",optional"`
		AppSecret string `json:",optional"`
	} `json:",optional"`
	// Audit 审计事件（outbox；Relay 属 S5 落地，本阶段仅事务内写表）。
	Audit struct {
		Enabled bool `json:",default=true"`
	} `json:",optional"`
	// ConfigKey configcenter etcd key（S4 起消费，ADR-10）。
	ConfigKey string `json:",optional"`
}
