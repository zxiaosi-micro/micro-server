package svc

import (
	"context"
	"fmt"

	"micro-server/services/contract/internal/config"
	"micro-server/services/contract/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext contract 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // contract_db 主库连接
	Models *Models       // 模型聚合
	// Snowflake ID 生成器（合同号/质保号/延保号/索赔号 = 前缀 + 雪花）。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）
}

// Models model 聚合。
type Models struct {
	Template    model.ContractTemplateModel  // 合同模板
	Contract    model.ContractModel          // 合同
	Target      model.ContractTargetModel    // 合同标的（设备/场站）
	File        model.ContractFileModel      // 合同附件
	Warranty    model.WarrantyModel          // 质保
	Extension   model.WarrantyExtensionModel // 延保订单
	Sla         model.SlaStrategyModel       // SLA 策略模板
	ContractSla model.ContractSlaModel       // 合同绑定的 SLA 配置
	Claim       model.ClaimModel             // 索赔单
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
			Template:    model.NewContractTemplateModel(conn, cacheConf),
			Contract:    model.NewContractModel(conn, cacheConf),
			Target:      model.NewContractTargetModel(conn, cacheConf),
			File:        model.NewContractFileModel(conn, cacheConf),
			Warranty:    model.NewWarrantyModel(conn, cacheConf),
			Extension:   model.NewWarrantyExtensionModel(conn, cacheConf),
			Sla:         model.NewSlaStrategyModel(conn, cacheConf),
			ContractSla: model.NewContractSlaModel(conn, cacheConf),
			Claim:       model.NewClaimModel(conn, cacheConf),
		},
	}

	logx.Info("contract 就绪：db=micro_contract")
	return sc
}
