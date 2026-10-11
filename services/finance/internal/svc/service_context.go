package svc

import (
	"context"
	"fmt"

	"micro-server/services/finance/internal/config"
	"micro-server/services/finance/internal/model"

	partyclient "micro-server/services/party/party"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
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
	// Party 返利规则源（dealer_ext.rebate_rule；未配置 = 返利结算不可用）。
	Party partyclient.Party
}

// Models model 聚合。
type Models struct {
	Payment       model.PaymentModel
	Refund        model.RefundModel
	Invoice       model.InvoiceModel
	ReconcileTask model.ReconcileTaskModel
	Rebate        model.RebateSettlementModel
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
			Rebate:        model.NewRebateSettlementModel(conn, cacheConf),
		},
		Gateway: NewGateway(c),
	}

	// PartyRpc（S10-03 返利规则源；缺失降级：返利结算报业务错误）
	if len(c.PartyRpc.Etcd.Hosts) > 0 || c.PartyRpc.Target != "" {
		sc.Party = partyclient.NewParty(zrpc.MustNewClient(c.PartyRpc,
			zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))))
	}

	logx.Info("finance 就绪：db=micro_finance gateway=", sc.Gateway.Mode(), " party=", sc.Party != nil)
	return sc
}
