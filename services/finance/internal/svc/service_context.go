package svc

import (
	"context"
	"fmt"

	"micro-server/services/finance/internal/config"
	"micro-server/services/finance/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext finance 服务根装配。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器（支付单号/退款号/发票号/任务号 = 前缀 + 雪花）。
	Snowflake  *snowflake.Node
	snowCancel func()

	// 渠道网关（dev 模拟 / 微信 v3 / 支付宝 RSA2，按配置装配）。
	Gateway *Gateway
}

// Models model 聚合。
type Models struct {
	Payment       model.PaymentModel
	Refund        model.RefundModel
	Invoice       model.InvoiceModel
	ReconcileTask model.ReconcileTaskModel
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
			Payment:       model.NewPaymentModel(conn, cacheConf),
			Refund:        model.NewRefundModel(conn, cacheConf),
			Invoice:       model.NewInvoiceModel(conn, cacheConf),
			ReconcileTask: model.NewReconcileTaskModel(conn, cacheConf),
		},
		Gateway: NewGateway(c),
	}

	logx.Info("finance 就绪：db=micro_finance gateway=", sc.Gateway.Mode())
	return sc
}
