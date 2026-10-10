package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
)

// Config admin-bff 配置（02 §3.4）。
type Config struct {
	rest.RestConf
	// IdentityRpc identity 服务客户端（dev 直连 Endpoints；prod 走 etcd Target）。
	IdentityRpc zrpc.RpcClientConf
	// S4 业务域 RPC 客户端（S4-05 接入；dev 直连 Endpoints）。
	PartyRpc        zrpc.RpcClientConf `json:",optional"`
	CatalogRpc      zrpc.RpcClientConf `json:",optional"`
	InventoryRpc    zrpc.RpcClientConf `json:",optional"`
	NotificationRpc zrpc.RpcClientConf `json:",optional"`
	AuditRpc        zrpc.RpcClientConf `json:",optional"`
	// S5 交易域 RPC 客户端（S5-02~04 接入；dev 直连 Endpoints）。
	OrderRpc    zrpc.RpcClientConf `json:",optional"`
	FinanceRpc  zrpc.RpcClientConf `json:",optional"`
	ContractRpc zrpc.RpcClientConf `json:",optional"`
	// S6 资产域 RPC 客户端（S6-01/02 接入）。
	DeviceRpc  zrpc.RpcClientConf `json:",optional"`
	StationRpc zrpc.RpcClientConf `json:",optional"`
	// Jwt 公钥目录（与 identity 同源 keygen 产出；验签按 kid 选钥）。
	Jwt struct {
		KeysDir string `json:",optional"`
	} `json:",optional"`
	// Sessionx 会话中心只读方（BFF 每请求 GET sess:{sid} / auth_cache:{uid}，DB4）。
	Sessionx struct {
		Addr     string `json:",default=127.0.0.1:26379"`
		Password string `json:",optional"`
		DB       int    `json:",default=4"`
	} `json:",optional"`
	// Authz 鉴权中间件配置（免鉴权 /auth/* /sso/*；FailClose 指令/财务路径 S5+ 登记）。
	Authz authz.Conf `json:",optional"`
}
