package svc

import (
	"context"
	"fmt"

	"micro-server/services/audit/internal/config"
	"micro-server/services/audit/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext audit 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // audit_db 主库连接
	Models *Models       // 模型聚合
	// Snowflake ID 生成器。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）
}

// Models model 聚合。
type Models struct {
	AuditLog model.AuditLogModel // 通用操作审计（append-only）
	CmdAudit model.CmdAuditModel // 指令独立审计
}

// NewServiceContext 组装根：雪花 ID 硬依赖（失败即 panic）。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Models:     &Models{AuditLog: model.NewAuditLogModel(conn, c.Cache), CmdAudit: model.NewCmdAuditModel(conn, c.Cache)},
		Snowflake:  node,
		snowCancel: snowCancel,
	}

	logx.Info("audit 就绪：db=audit_db append_only")
	return sc
}
