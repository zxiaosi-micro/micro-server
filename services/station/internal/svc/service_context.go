// ServiceContext station 服务根装配（S6-02）。
package svc

import (
	"context"
	"fmt"

	"micro-server/services/station/internal/config"
	"micro-server/services/station/internal/model"

	devclient "micro-server/services/device/device"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
)

// ServiceContext station 服务根装配。
type ServiceContext struct {
	Config config.Config // 全量配置
	Conn   sqlx.SqlConn  // station_db 主库连接
	Models *Models       // 模型聚合
	// Snowflake ID 生成器（场站/绑定/拓扑 ID）。
	Snowflake  *snowflake.Node
	snowCancel func() // worker_id 租约释放（停机时调用）

	// ShadowRd 设备影子只读（Redis DB0；键 micro:iot:shadow:{sn}，writer=iotingest）。
	ShadowRd *redis.Redis
	// Device 设备 RPC（sn→device_id 归一；未配置 = 仅支持 device_id 绑定）。
	Device devclient.Device
}

// Models model 聚合。
type Models struct {
	Station  model.StationModel         // 场站
	Device   model.StationDeviceModel   // 场站-设备绑定
	Staff    model.StationStaffModel    // 场站员工
	Topology model.StationTopologyModel // 场站拓扑
}

// NewServiceContext 组装根：雪花 ID 硬依赖（失败即 panic）；影子 Redis/device RPC 软依赖（缺省降级）。
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
		Models: &Models{
			Station:  model.NewStationModel(conn, c.Cache),
			Device:   model.NewStationDeviceModel(conn, c.Cache),
			Staff:    model.NewStationStaffModel(conn, c.Cache),
			Topology: model.NewStationTopologyModel(conn, c.Cache),
		},
	}

	// 影子只读连接
	if c.ShadowRedis.Host != "" {
		if rd, rerr := redis.NewRedis(c.ShadowRedis); rerr == nil {
			sc.ShadowRd = rd
		} else {
			logx.Errorf("station: 影子 Redis 构造失败（监控聚合降级）: %v", rerr)
		}
	}

	// device RPC（sn 归一 + 反查）
	if len(c.DeviceRpc.Endpoints) > 0 || c.DeviceRpc.Target != "" {
		sc.Device = devclient.NewDevice(zrpc.MustNewClient(c.DeviceRpc,
			zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))))
	}

	logx.Infof("station 就绪：db=micro_station shadow=%v device_rpc=%v",
		sc.ShadowRd != nil, sc.Device != nil)
	return sc
}
