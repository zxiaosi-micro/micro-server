// ServiceContext device 服务根装配（S6-01）。
package svc

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/zeromicro/go-zero/zrpc"

	"micro-server/services/device/internal/config"
	"micro-server/services/device/internal/emqxadmin"
	"micro-server/services/device/internal/model"
	"micro-server/services/device/internal/shadow"

	auclient "micro-server/services/audit/audit"

	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
)

// ServiceContext device 服务根装配。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器（任务号/指令 ID/固件 ID）。
	Snowflake  *snowflake.Node
	snowCancel func()

	// Rd 影子 Redis（DB0）。
	Rd *redis.Redis
	// Shadow 影子读取端。
	Shadow *shadow.Store
	// Emqx EMQX 管理面 + 指令下行 MQTT（未配置 = 降级）。
	Emqx *emqxadmin.Client
	// AckWheel 指令 ACK 超时轮（TimingWheel 内存态，02 §9.6；cron 扫描兜底持久态）。
	AckWheel *collection.TimingWheel
	// Encryptor 设备密钥 AES-256-GCM（MICRO_DATA_KEYS）。
	Encryptor *crypto.Encryptor
	// OtaPubKey Ed25519 固件验签公钥（未配置 = SaveFirmware 拒绝入库）。
	OtaPubKey []byte

	// Audit 指令独立审计（audit.WriteCmdLog；未配置跳过）。
	Audit auclient.Audit
}

// Models model 聚合。
type Models struct {
	Device       model.DeviceModel
	LifecycleLog model.DeviceLifecycleLogModel
	Topology     model.DeviceTopologyModel
	Firmware     model.FirmwareModel
	OtaTask      model.OtaTaskModel
	OtaDevice    model.OtaDeviceModel
	Cmd          model.CmdModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Snowflake:  node,
		snowCancel: snowCancel,
		Models: &Models{
			Device:       model.NewDeviceModel(conn, c.Cache),
			LifecycleLog: model.NewDeviceLifecycleLogModel(conn, c.Cache),
			Topology:     model.NewDeviceTopologyModel(conn, c.Cache),
			Firmware:     model.NewFirmwareModel(conn, c.Cache),
			OtaTask:      model.NewOtaTaskModel(conn, c.Cache),
			OtaDevice:    model.NewOtaDeviceModel(conn, c.Cache),
			Cmd:          model.NewCmdModel(conn, c.Cache),
		},
	}

	// 影子 Redis（DB0）
	if c.ShadowRedis.Host != "" {
		rd, rerr := redis.NewRedis(c.ShadowRedis)
		if rerr == nil {
			sc.Rd = rd
			sc.Shadow = shadow.NewStore(rd)
		} else {
			logx.Errorf("device: 影子 Redis 构造失败（影子查询不可用）: %v", rerr)
		}
	}

	// EMQX 管理面 + MQTT（降级模式：未配置仅记日志）
	sc.Emqx = emqxadmin.New(c.Emqx.ApiBase, c.Emqx.DashboardUser, c.Emqx.DashboardPass,
		c.Emqx.Broker, c.Emqx.PlatformUser, c.Emqx.PlatformPass)

	// ACK TimingWheel（interval=100ms，slots=64 覆盖 6.4s；3s 计 ACK，02 §9.6）
	wheel, werr := collection.NewTimingWheel(100*time.Millisecond, 64, func(key, value any) {
		if fn, ok := value.(func()); ok {
			fn()
		}
	})
	if werr != nil {
		panic(fmt.Errorf("ACK TimingWheel 构造失败: %w", werr))
	}
	sc.AckWheel = wheel

	// 数据密钥（设备 secret 加密；未配置 = ImportSN/ProvisionCredential 报错）
	if enc, eerr := crypto.EnvKeyProviderFromEnv(); eerr == nil {
		if e, cerr := crypto.NewEncryptor(enc); cerr == nil {
			sc.Encryptor = e
		} else {
			logx.Errorf("device: 加密器构造失败: %v", cerr)
		}
	} else {
		logx.Errorf("device: MICRO_DATA_KEYS 未配置（设备密钥加密不可用）: %v", eerr)
	}

	// OTA Ed25519 公钥（未配置 = 固件拒绝入库）
	if c.OtaPublicKeyPath != "" {
		if pk, perr := loadPemPubKey(c.OtaPublicKeyPath); perr == nil {
			sc.OtaPubKey = pk
		} else {
			logx.Errorf("device: OTA 公钥加载失败（固件入库将拒绝）: %v", perr)
		}
	} else {
		logx.Info("device: OTA_PUBLIC_KEY 未配置（固件签名校验将拒绝入库）")
	}

	// 指令审计（audit RPC）
	if len(c.AuditRpc.Endpoints) > 0 || c.AuditRpc.Target != "" {
		sc.Audit = auclient.NewAudit(zrpcMustClient(c.AuditRpc))
	}

	logx.Infof("device 就绪：db=micro_device shadow=%v emqx_api=%v mqtt=%v ota_key=%v audit=%v",
		sc.Shadow != nil, !sc.Emqx.Degraded(), c.Emqx.Broker != "", len(sc.OtaPubKey) > 0, sc.Audit != nil)
	return sc
}

func zrpcMustClient(c zrpc.RpcClientConf) zrpc.Client {
	return zrpc.MustNewClient(c, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
}

// loadPemPubKey 读取 PKIX PEM Ed25519 公钥（keygen ota_ed25519_public.pem）。
func loadPemPubKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("PEM 解码失败: %s", path)
	}
	key, err := parsePKIXEd25519(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func parsePKIXEd25519(der []byte) ([]byte, error) {
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	k, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("非 Ed25519 公钥")
	}
	return []byte(k), nil
}
