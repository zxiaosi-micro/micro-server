package svc

import (
	"context"
	"fmt"

	"micro-server/services/catalog/internal/config"
	"micro-server/services/catalog/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext catalog 服务根装配。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Snowflake ID 生成器（etcd 租约分配 worker_id，退化 MICRO_WORKER_ID 环境变量）。
	Snowflake  *snowflake.Node
	snowCancel func()
}

// Models model 聚合。
type Models struct {
	Product           model.ProductModel
	Sku               model.SkuModel
	Price             model.PriceModel
	WarrantyPolicy    model.WarrantyPolicyModel
	StationProduct    model.StationProductModel
	StationProductMdl model.StationProductItemModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	// 雪花 ID：etcd 租约分配 worker_id（无端点则退化 MICRO_WORKER_ID 环境变量）
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
			Product:           model.NewProductModel(conn, cacheConf),
			Sku:               model.NewSkuModel(conn, cacheConf),
			Price:             model.NewPriceModel(conn, cacheConf),
			WarrantyPolicy:    model.NewWarrantyPolicyModel(conn, cacheConf),
			StationProduct:    model.NewStationProductModel(conn, cacheConf),
			StationProductMdl: model.NewStationProductItemModel(conn, cacheConf),
		},
	}

	logx.Info("catalog 就绪：db=catalog_db snowflake=etcd")
	return sc
}
