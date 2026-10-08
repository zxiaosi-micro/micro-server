package svc

import (
	"crypto/rsa"
	"encoding/json"
	"os"

	"micro-server/services/admin-bff/internal/config"
	apb "micro-server/services/audit/pb"
	cpb "micro-server/services/catalog/pb"
	ctpb "micro-server/services/contract/pb"
	finpb "micro-server/services/finance/pb"
	ipb "micro-server/services/identity/pb"
	invpb "micro-server/services/inventory/pb"
	npb "micro-server/services/notification/pb"
	opb "micro-server/services/order/pb"
	ppb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
	"github.com/zxiaosi-micro/micro-common/sessionx"
	"google.golang.org/grpc"
)

// ServiceContext admin-bff 根装配。
type ServiceContext struct {
	Config   config.Config
	Authz    rest.Middleware    // goctl @server middleware: Authz 挂载点（az.Handle）
	Identity ipb.IdentityClient // identity RPC 客户端
	// S4 业务域 RPC 客户端（dev 直连 Endpoints；出站统一挂 metadata 桥）
	Party        ppb.PartyClient
	Catalog      cpb.CatalogClient
	Inventory    invpb.InventoryClient
	Notification npb.NotificationClient
	Audit        apb.AuditClient
	// S5 交易域 RPC 客户端
	Order    opb.OrderClient
	Finance  finpb.FinanceClient
	Contract ctpb.ContractClient
	Verifier *jwtauth.Verifier // 免鉴权组（logout/step-up）解析 Bearer 取 sid
	sessions *sessionx.Store   // 会话中心只读方（预留：登出后本地校验）
}

func NewServiceContext(c config.Config) *ServiceContext {
	// RPC 客户端统一构造：出站挂 metadata 桥（ctxkit → x-micro-*，业务码无损往返，02 §9.5）。
	// 用原生 gRPC dial option（go-zero 内置 client 中间件链不透传自定义拦截器）。
	dial := func(conf zrpc.RpcClientConf) grpc.ClientConnInterface {
		conn := zrpc.MustNewClient(conf,
			zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
		return conn.Conn()
	}

	// 会话中心只读方（sess:{sid} 会话校验 + auth_cache 每请求 GET，Redis DB4）
	store, err := sessionx.New(sessionx.Conf{
		Addr:     c.Sessionx.Addr,
		Password: c.Sessionx.Password,
		DB:       c.Sessionx.DB,
	})
	if err != nil {
		panic(err)
	}

	verifier := loadVerifier(c.Jwt.KeysDir)

	az, err := authz.New(verifier, store, store, c.Authz,
		authz.WithPermResolver(permResolver),
		authz.WithDegradeHook(func(reason string) {
			logx.Errorf("[authz 降级告警] %s", reason)
		}),
	)
	if err != nil {
		panic(err)
	}

	return &ServiceContext{
		Config:       c,
		Authz:        az.Handle,
		Identity:     ipb.NewIdentityClient(dial(c.IdentityRpc)),
		Party:        ppb.NewPartyClient(dial(c.PartyRpc)),
		Catalog:      cpb.NewCatalogClient(dial(c.CatalogRpc)),
		Inventory:    invpb.NewInventoryClient(dial(c.InventoryRpc)),
		Notification: npb.NewNotificationClient(dial(c.NotificationRpc)),
		Audit:        apb.NewAuditClient(dial(c.AuditRpc)),
		Order:        opb.NewOrderClient(dial(c.OrderRpc)),
		Finance:      finpb.NewFinanceClient(dial(c.FinanceRpc)),
		Contract:     ctpb.NewContractClient(dial(c.ContractRpc)),
		Verifier:     verifier,
		sessions:     store,
	}
}

// loadVerifier 从 keygen 密钥目录装配公钥集合（与 identity 同源 keys.json + 公钥 PEM；
// 轮换期目录内追加旧公钥 PEM 并扩 keys.json 多钥集合即可，BFF 侧零改动）。
func loadVerifier(keysDir string) *jwtauth.Verifier {
	if keysDir == "" {
		keysDir = os.Getenv("JWT_KEYS_DIR")
	}
	raw, err := os.ReadFile(keysDir + "/keys.json")
	if err != nil {
		panic(err)
	}
	var meta struct {
		JWT struct {
			KID    string `json:"kid"`
			Public string `json:"public"`
		} `json:"jwt"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		panic(err)
	}
	pubName := meta.JWT.Public
	if pubName == "" {
		pubName = "jwt_rs256_public.pem"
	}
	pemBytes, err := os.ReadFile(keysDir + "/" + pubName)
	if err != nil {
		panic(err)
	}
	pub, err := jwtauth.ParsePublicKeyPEM(pemBytes)
	if err != nil {
		panic(err)
	}
	v, err := jwtauth.NewVerifier(map[string]*rsa.PublicKey{meta.JWT.KID: pub})
	if err != nil {
		panic(err)
	}
	return v
}
