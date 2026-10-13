package svc

import (
	"context"
	"fmt"
	"os"

	"micro-server/services/party/internal/config"
	"micro-server/services/party/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext party 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // party_db 主库连接
	Models *Models       // 模型聚合
	// Snowflake ID 生成器（etcd 租约分配 worker_id，退化 MICRO_WORKER_ID 环境变量）。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）
	// Encryptor contact.mobile AES-256-GCM 加密（MICRO_DATA_KEYS 注入）。
	Encryptor *crypto.Encryptor
	// HashKey contact.mobile_hash HMAC-SHA256 索引键。
	HashKey []byte
}

// Models model 聚合。
type Models struct {
	Party       model.PartyModel       // 参与方（buyer/supplier/dealer/platform）
	Contact     model.ContactModel     // 联系方式（手机号加密存储）
	Staff       model.StaffModel       // 员工（uid ↔ party 绑定）
	DealerExt   model.DealerExtModel   // 经销商扩展档案
	CrmRecord   model.CrmRecordModel   // CRM 跟进记录
	Opportunity model.OpportunityModel // 销售机会
}

// NewServiceContext 组装根：雪花 ID/数据加密器硬依赖（失败即 panic）。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	// 雪花 ID：etcd 租约分配 worker_id（无端点则退化 MICRO_WORKER_ID 环境变量）
	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	enc, err := loadEncryptor()
	if err != nil {
		panic(err)
	}

	cacheConf := c.Cache
	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Snowflake:  node,
		snowCancel: snowCancel,
		Encryptor:  enc,
		HashKey:    hmacKey(c.HashKeyEnv),
		Models: &Models{
			Party:       model.NewPartyModel(conn, cacheConf),
			Contact:     model.NewContactModel(conn, cacheConf),
			Staff:       model.NewStaffModel(conn, cacheConf),
			DealerExt:   model.NewDealerExtModel(conn, cacheConf),
			CrmRecord:   model.NewCrmRecordModel(conn, cacheConf),
			Opportunity: model.NewOpportunityModel(conn, cacheConf),
		},
	}

	logx.Info("party 就绪：db=party_db snowflake=etcd")
	return sc
}

// loadEncryptor 装配数据加密器（MICRO_DATA_KEYS / MICRO_DATA_KEY_KID 环境变量注入）。
func loadEncryptor() (*crypto.Encryptor, error) {
	provider, err := crypto.EnvKeyProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("数据密钥未就绪（keygen 产出后注入 MICRO_DATA_KEYS）: %w", err)
	}
	return crypto.NewEncryptor(provider)
}

// hmacKey 取 HMAC-SHA256 索引键（与 identity/seed 同口径）。
func hmacKey(envName string) []byte {
	if v := os.Getenv(envName); v != "" {
		return []byte(v)
	}
	return []byte("micro-dev-index-key")
}
