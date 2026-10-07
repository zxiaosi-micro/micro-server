package svc

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/identity/internal/config"
	"micro-server/services/identity/internal/keys"
	"micro-server/services/identity/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
	"github.com/zxiaosi-micro/micro-common/sessionx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext identity 服务根装配。
type ServiceContext struct {
	Config    config.Config
	Conn      sqlx.SqlConn
	Models    *Models
	Sessions  *sessionx.Store
	Signer    *jwtauth.Signer
	Verifier  *jwtauth.Verifier // 供 ValidateSession 重签/自验场景
	Encryptor *crypto.Encryptor
	HashKey   []byte
	// Snowflake ID 生成器（etcd 租约分配 worker_id，退化 MICRO_WORKER_ID 环境变量）。
	Snowflake  *snowflake.Node
	snowCancel func()
	AccessTTL  int64 // access token 有效期秒数（LoginResp 过期回显）
	RefreshTTL int64 // refresh token 有效期秒数
	SessionTTL time.Duration
}

// Models model 聚合（goctl 生成 + custom 覆写 + 手写绑定表）。
type Models struct {
	Tenant model.TenantModel
	User   model.UserModel
	Org    model.OrgModel
	Role   model.RoleModel
	Menu   model.MenuModel
	// 绑定表（联合主键，手写 model）
	UserRole model.UserRoleModel
	RoleMenu model.RoleMenuModel
	// SSO
	SsoBinding model.UserSsoBindingModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	// 会话中心：硬依赖，启动期连不通直接 panic（sessionx.New 内 Ping）
	store, err := sessionx.New(sessionx.Conf{
		Addr:       c.Sessionx.Addr,
		Password:   c.Sessionx.Password,
		DB:         c.Sessionx.DB,
		SessionTTL: c.Sessionx.SessionTTL,
		RefreshTTL: c.Sessionx.RefreshTTL,
		StepUpTTL:  c.Sessionx.StepUpTTL,
	})
	if err != nil {
		panic(err)
	}

	signer, verifier, err := keys.LoadJWT(c.Jwt.KeysDir, c.Jwt.PrivateKeyFile, c.Jwt.PublicKeyFile)
	if err != nil {
		panic(err)
	}

	enc, err := keys.LoadDataEncryptor()
	if err != nil {
		panic(err)
	}

	// 雪花 ID：etcd 租约分配 worker_id（无端点则退化 MICRO_WORKER_ID 环境变量）
	etcdHosts := c.Etcd.Hosts
	node, snowCancel, err := snowflake.NewAuto(context.Background(), etcdHosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	// goctl model 缓存：Cache 段缺省时退化为无缓存直连（行缓存仅主键 FindOne 受益）
	cacheConf := c.Cache

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Sessions:   store,
		Signer:     signer,
		Verifier:   verifier,
		Encryptor:  enc,
		HashKey:    keys.HMACKey(c.HashKeyEnv),
		Snowflake:  node,
		snowCancel: snowCancel,
		AccessTTL:  int64(c.Jwt.AccessTTL.Seconds()),
		RefreshTTL: int64(c.Sessionx.RefreshTTL.Seconds()),
		SessionTTL: c.Sessionx.SessionTTL,
		Models: &Models{
			Tenant:     model.NewTenantModel(conn, cacheConf),
			User:       model.NewUserModel(conn, cacheConf),
			Org:        model.NewOrgModel(conn, cacheConf),
			Role:       model.NewRoleModel(conn, cacheConf),
			Menu:       model.NewMenuModel(conn, cacheConf),
			UserRole:   model.NewUserRoleModel(conn),
			RoleMenu:   model.NewRoleMenuModel(conn),
			SsoBinding: model.NewUserSsoBindingModel(conn, cacheConf),
		},
	}

	logx.Infof("identity 就绪：kid=%s sessionx=%s db4 audit=%v",
		signer.KID(), c.Sessionx.Addr, c.Audit.Enabled)
	return sc
}
