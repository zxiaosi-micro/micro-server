// ServiceContext · hello 根装配（S3-06 模板：model/cron/configcenter 的装配位）。

package svc

import (
	"micro-server/services/hello/internal/config"
	"micro-server/services/hello/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext hello 服务根。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Model  model.GreetingModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)
	return &ServiceContext{
		Config: c,
		Conn:   conn,
		Model:  model.NewGreetingModel(conn, c.Cache),
	}
}
