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
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器。
	Snowflake  *snowflake.Node
	snowCancel func()
}

// Models model 聚合。
type Models struct {
	Message     model.MessageModel
	Template    model.NotifyTemplateModel
	Channel     model.NotifyChannelModel
	Outbound    model.OutboundLogModel
	UserSetting model.UserNotifySettingModel
}

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
