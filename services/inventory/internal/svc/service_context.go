package svc

import (
	"context"
	"fmt"

	"micro-server/services/inventory/internal/config"
	"micro-server/services/inventory/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/snowflake"
)

// ServiceContext inventory 服务根装配。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *Models
	// Rd 防超卖预扣网关（02 §9.2 双保险第一层；nil/故障时降级纯 DB——同样不超卖）。
	Rd *redis.Redis
	// Snowflake ID 生成器（etcd 租约分配 worker_id，退化 MICRO_WORKER_ID 环境变量）。
	Snowflake  *snowflake.Node
	snowCancel func()
}

// Models model 聚合。
type Models struct {
	Warehouse     model.WarehouseModel
	Inventory     model.InventoryModel
	StockRecord   model.StockRecordModel
	Stocktake     model.StocktakeModel
	StocktakeItem model.StocktakeItemModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	// 雪花 ID：etcd 租约分配 worker_id（无端点则退化 MICRO_WORKER_ID 环境变量）
	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	// Redis 预扣网关：复用 Cache 段首节点；未配置时为 nil（纯 DB 路径，02 §9.2 第 4 步）
	var rd *redis.Redis
	if len(c.Cache) > 0 && c.RedisGate.Enabled {
		rd = redis.MustNewRedis(redis.RedisConf{
			Host: c.Cache[0].Host,
			Pass: c.Cache[0].Pass,
			Type: "node",
		})
	}

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Models:     newModels(conn, c.Cache),
		Rd:         rd,
		Snowflake:  node,
		snowCancel: snowCancel,
	}

	logx.Infof("inventory 就绪：db=inventory_db redis_gate=%v", rd != nil)
	return sc
}

func newModels(conn sqlx.SqlConn, cacheConf cache.CacheConf) *Models {
	return &Models{
		Warehouse:     model.NewWarehouseModel(conn, cacheConf),
		Inventory:     model.NewInventoryModel(conn, cacheConf),
		StockRecord:   model.NewStockRecordModel(conn, cacheConf),
		Stocktake:     model.NewStocktakeModel(conn, cacheConf),
		StocktakeItem: model.NewStocktakeItemModel(conn, cacheConf),
	}
}
