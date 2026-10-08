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
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器（合同号/质保号/延保号/索赔号 = 前缀 + 雪花）。
	Snowflake  *snowflake.Node
	snowCancel func()
}

// Models model 聚合。
type Models struct {
	Template   model.ContractTemplateModel
	Contract   model.ContractModel
	Target     model.ContractTargetModel
	File       model.ContractFileModel
	Warranty   model.WarrantyModel
	Extension  model.WarrantyExtensionModel
	Sla        model.SlaStrategyModel
	ContractSla model.ContractSlaModel
	Claim      model.ClaimModel
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
