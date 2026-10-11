package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
)

// Config mobile-bff 配置（S9-01，02 §3.4）。BFF 不监听配置中心（与 admin-bff 同口径）。
type Config struct {
	rest.RestConf
	// IdentityRpc 认证/会话（唯一写入方；BFF 只读 sessionx）。
	IdentityRpc zrpc.RpcClientConf
	// InventoryRpc 延保下单代选仓库（只读 ListWarehouse）。
	InventoryRpc zrpc.RpcClientConf `json:",optional"`
	// S9 业务域 RPC 客户端（requireParty 查询与端侧聚合；dev 直连 Endpoints）。
	PartyRpc        zrpc.RpcClientConf `json:",optional"`
	CatalogRpc      zrpc.RpcClientConf `json:",optional"`
	OrderRpc        zrpc.RpcClientConf `json:",optional"`
	ContractRpc     zrpc.RpcClientConf `json:",optional"`
	DeviceRpc       zrpc.RpcClientConf `json:",optional"`
	StationRpc      zrpc.RpcClientConf `json:",optional"`
	OpsRpc          zrpc.RpcClientConf `json:",optional"`
	NotificationRpc zrpc.RpcClientConf `json:",optional"`
	// Jwt 公钥目录（与 identity 同源 keygen 产出；验签按 kid 选钥）。
	Jwt struct {
		KeysDir string `json:",optional"`
	} `json:",optional"`
	// Sessionx 会话中心只读方（sess:{sid} / auth_cache:{uid}，Redis DB4）。
	Sessionx struct {
		Addr     string `json:",default=127.0.0.1:26379"`
		Password string `json:",optional"`
		DB       int    `json:",default=4"`
	} `json:",optional"`
	// Authz 鉴权中间件（AllowedClients=[OPS_APP,CLIENT_MINI]；延保下单属财务面 fail-close）。
	Authz authz.Conf `json:",optional"`
	// Tdengine 遥测趋势只读直查（REST + Basic 认证，E7；仅 SELECT，业务库零直连）。
	Tdengine struct {
		RestUrl string `json:",default=http://127.0.0.1:26041"`
		User    string `json:",default=root"`
		Pass    string `json:",optional"`
		DB      string `json:",default=micro_iot"`
	} `json:",optional"`
}
