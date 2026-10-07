package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
)

// Config hello 配置（02 §3.4：结构体唯一契约；${ENV} 注入敏感值）。
type Config struct {
	rest.RestConf
	// Mysql 数据源（模板演示 custom model 全链）。
	Mysql struct {
		DataSource string
	} `json:",optional"`
	// Cache goctl model 行缓存。
	Cache cache.CacheConf `json:",optional"`
	// Etcd 服务注册 + configcenter 订阅源。
	Etcd struct {
		Hosts []string
		Key   string
	} `json:",optional"`
	// ConfigKey configcenter etcd key（运行时参数源，ADR-10）。
	ConfigKey string `json:",default=/micro/config/hello"`
}
