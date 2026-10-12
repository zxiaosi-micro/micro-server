package svc

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	cpb "micro-server/services/catalog/pb"
	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	ipb "micro-server/services/identity/pb"
	invpb "micro-server/services/inventory/pb"
	"micro-server/services/mobile-bff/internal/config"
	npb "micro-server/services/notification/pb"
	opsb "micro-server/services/ops/pb"
	opb "micro-server/services/order/pb"
	ppb "micro-server/services/party/pb"
	spb "micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
	"github.com/zxiaosi-micro/micro-common/sessionx"
	"google.golang.org/grpc"
)

// ServiceContext mobile-bff 根装配（S9-01）。
type ServiceContext struct {
	Config    config.Config
	Authz     rest.Middleware // goctl @server middleware: Authz 挂载点（az.Handle）
	Identity  ipb.IdentityClient
	Inventory invpb.InventoryClient
	Party     ppb.PartyClient
	Catalog   cpb.CatalogClient
	Order     opb.OrderClient
	Contract  ctpb.ContractClient
	Device    dpb.DeviceClient
	Station   spb.StationClient
	Ops       opsb.OpsClient
	Notify    npb.NotificationClient
	Verifier  *jwtauth.Verifier // 免鉴权组（logout）解析 Bearer 取 sid
	Td        *TdClient         // 遥测趋势只读直查（REST/Basic，E7）
	sessions  *sessionx.Store   // 会话中心只读方（预留）
	QuotaMW   rest.Middleware   // 租户 RPM 配额限流（S11-01；全局 Use，先于路由级 authz）
}

func NewServiceContext(c config.Config) *ServiceContext {
	// RPC 客户端统一构造：出站挂 metadata 桥（ctxkit → x-micro-*，业务码无损往返，02 §9.5）。
	dial := func(conf zrpc.RpcClientConf) grpc.ClientConnInterface {
		conn := zrpc.MustNewClient(conf,
			zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
		return conn.Conn()
	}

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

	identity := ipb.NewIdentityClient(dial(c.IdentityRpc))
	quotaMW := Quota(NewQuotaLimiter(c.Sessionx.Addr, c.Sessionx.Password, identity), verifier, store)

	return &ServiceContext{
		Config:    c,
		Authz:     az.Handle,
		Identity:  identity,
		Inventory: invpb.NewInventoryClient(dial(c.InventoryRpc)),
		Party:     ppb.NewPartyClient(dial(c.PartyRpc)),
		Catalog:   cpb.NewCatalogClient(dial(c.CatalogRpc)),
		Order:     opb.NewOrderClient(dial(c.OrderRpc)),
		Contract:  ctpb.NewContractClient(dial(c.ContractRpc)),
		Device:    dpb.NewDeviceClient(dial(c.DeviceRpc)),
		Station:   spb.NewStationClient(dial(c.StationRpc)),
		Ops:       opsb.NewOpsClient(dial(c.OpsRpc)),
		Notify:    npb.NewNotificationClient(dial(c.NotificationRpc)),
		Verifier:  verifier,
		Td:        NewTdClient(c.Tdengine.RestUrl, c.Tdengine.User, c.Tdengine.Pass, c.Tdengine.DB),
		sessions:  store,
		QuotaMW:   quotaMW,
	}
}

// loadVerifier 从 keygen 密钥目录装配公钥集合（与 admin-bff 同款；轮换期目录追加旧公钥即可）。
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

// BearerProbe 免鉴权组（/auth/logout /auth/refresh）的 Bearer 探针：
// 有合法 token 则注入 uid/sid/client 到 ctx（不查会话、不判权限）。
func BearerProbe(verifier *jwtauth.Verifier) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if len(auth) >= 8 && strings.EqualFold(auth[:7], "Bearer ") {
				if claims, err := verifier.Verify(strings.TrimSpace(auth[7:])); err == nil {
					ctx := ctxkitWithUIDSID(r.Context(), claims)
					r = r.WithContext(ctx)
				}
			}
			next(w, r)
		}
	}
}

// permResolver mobile-bff 端侧路由全部登录即可（C 端/运维个人面无按钮级 RBAC；
// 数据隔离靠 requireParty / assignee 过滤，越权由下游数据域拒绝）。
func permResolver(r *http.Request) (string, bool) { return "", false }
