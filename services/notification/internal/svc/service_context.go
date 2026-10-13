package svc

import (
	"context"
	"fmt"

	"micro-server/services/notification/internal/config"
	"micro-server/services/notification/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext notification 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // notification_db 主库连接
	Models *Models       // 模型聚合
	// Snowflake ID 生成器。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）
}

// Models model 聚合。
type Models struct {
	Message     model.MessageModel           // 站内信
	Template    model.NotifyTemplateModel    // 通知模板
	Channel     model.NotifyChannelModel     // 外发渠道配置
	Outbound    model.OutboundLogModel       // 外发触达日志
	UserSetting model.UserNotifySettingModel // 用户通知设置（启用/免打扰）
}

// NewServiceContext 组装根：雪花 ID 硬依赖（失败即 panic）。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	cacheConf := c.Cache
	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Snowflake:  node,
		snowCancel: snowCancel,
		Models: &Models{
			Message:     model.NewMessageModel(conn, cacheConf),
			Template:    model.NewNotifyTemplateModel(conn, cacheConf),
			Channel:     model.NewNotifyChannelModel(conn, cacheConf),
			Outbound:    model.NewOutboundLogModel(conn, cacheConf),
			UserSetting: model.NewUserNotifySettingModel(conn, cacheConf),
		},
	}

	logx.Info("notification 就绪：db=notification_db provider=log-stub")
	return sc
}
