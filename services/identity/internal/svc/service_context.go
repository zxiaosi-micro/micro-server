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
	Config    config.Config     // 全量配置
	Conn      sqlx.SqlConn      // identity_db 主库连接
	Models    *Models           // 模型聚合
	Sessions  *sessionx.Store   // 会话中心（Redis DB4，唯一读写方）
	Signer    *jwtauth.Signer   // JWT 签发器（RS256，kid 轮换）
	Verifier  *jwtauth.Verifier // 供 ValidateSession 重签/自验场景
	Encryptor *crypto.Encryptor // 手机号等敏感字段加解密器
	HashKey   []byte            // HMAC 密钥（手机号摘要查重用）
	// Snowflake ID 生成器（etcd 租约分配 worker_id，退化 MICRO_WORKER_ID 环境变量）。
	Snowflake  *snowflake.Node
	snowCancel func()        // worker_id 租约释放（停机时调用）
	AccessTTL  int64         // access token 有效期秒数（LoginResp 过期回显）
	RefreshTTL int64         // refresh token 有效期秒数
	SessionTTL time.Duration // 会话中心会话有效期
}

// Models model 聚合（goctl 生成 + custom 覆写 + 手写绑定表）。
type Models struct {
	Tenant model.TenantModel // 租户表
	User   model.UserModel   // 用户表
	Org    model.OrgModel    // 组织表
	Role   model.RoleModel   // 角色表
	Menu   model.MenuModel   // 菜单/权限表
	// 绑定表（联合主键，手写 model）
	UserRole model.UserRoleModel // 用户-角色绑定
	RoleMenu model.RoleMenuModel // 角色-菜单授权
	// SSO
	SsoBinding model.UserSsoBindingModel // SSO 外部身份绑定
}

// NewServiceContext 组装根：连接/会话中心/JWT 密钥/数据加密/雪花 ID 全是硬依赖，
// 任一初始化失败直接 panic（启动期失败优于带病运行）。
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
