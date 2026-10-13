package svc

import (
	"context"
	"fmt"

	"micro-server/services/ops/internal/config"
	"micro-server/services/ops/internal/model"

	contractclient "micro-server/services/contract/contract"
	partyclient "micro-server/services/party/party"
	stationclient "micro-server/services/station/station"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
)

// ServiceContext ops 服务根装配（S7-01）。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // ops_db 主库连接
	Models *model.Models // 模型聚合
	// Snowflake ID 生成器（告警/工单/计时器/巡检全族 ID）。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）

	// Rd 轮询未读计数（Redis node；与 sessionx DB4 无关）。
	Rd *redis.Redis
	// Station 设备→场站归属反查（告警聚合键必需）。
	Station stationclient.Station
	// Party 派单候选（ListStaff 区域+技能匹配）。
	Party partyclient.Party
	// Contract SLA 策略正本（ListSlaStrategy 透传 + 快照冻结）。
	Contract contractclient.Contract
}

// NewServiceContext 组装根：雪花 ID 硬依赖（失败即 panic）；
// Redis/下游 RPC 软依赖（缺失降级：告警落平台级、派单仅手动、SLA 不计时）。
func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	sc := &ServiceContext{
		Config:     c,
		Conn:       conn,
		Snowflake:  node,
		snowCancel: snowCancel,
		Models:     model.NewModels(conn),
	}

	// 轮询未读计数（未配置 Redis = 直查 DB，功能不缺）
	if len(c.Cache) > 0 {
		if rd, rerr := redis.NewRedis(c.Cache[0].RedisConf); rerr == nil {
			sc.Rd = rd
		} else {
			logx.Errorf("ops: Redis 构造失败（未读计数直查 DB）: %v", rerr)
		}
	}

	// RPC 客户端（缺失降级：station 缺→告警落平台级；party 缺→派单仅手动；contract 缺→SLA 不计时）
	sc.Station = mustClient(c.StationRpc, "station", stationclient.NewStation)
	sc.Party = mustClient(c.PartyRpc, "party", partyclient.NewParty)
	sc.Contract = mustClient(c.ContractRpc, "contract", contractclient.NewContract)

	logx.Infof("ops 就绪：db=micro_ops station_rpc=%v party_rpc=%v contract_rpc=%v",
		sc.Station != nil, sc.Party != nil, sc.Contract != nil)
	return sc
}

// mustClient RPC 客户端装配（配置缺失返回 nil，调用方按 nil 降级）。
func mustClient[T any](conf zrpc.RpcClientConf, name string, New func(zrpc.Client) T) T {
	var zero T
	if len(conf.Etcd.Hosts) == 0 && conf.Target == "" {
		logx.Errorf("ops: %s RPC 未配置（相关能力降级）", name)
		return zero
	}
	cli := zrpc.MustNewClient(conf, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)))
	return New(cli)
}
