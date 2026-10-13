// ServiceContext · hello 根装配（S3-06 模板：model/cron/configcenter 的装配位）。

package svc

import (
	"micro-server/services/hello/internal/config"
	"micro-server/services/hello/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext hello 服务根。
type ServiceContext struct {
	Config config.Config       // 全量配置
	Conn   sqlx.SqlConn        // hello_db 主库连接
	Model  model.GreetingModel // 示例表模型
}

// NewServiceContext 组装根（新服务骨架照抄入口）。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	return &ServiceContext{
		Config: c,
		Conn:   conn,
		Model:  model.NewGreetingModel(conn, c.Cache),
	}
}
