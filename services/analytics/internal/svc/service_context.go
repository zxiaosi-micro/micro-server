package svc

import (
	"context"
	"fmt"

	"micro-server/services/analytics/internal/config"
	"micro-server/services/analytics/internal/model"

	fileclient "micro-server/services/file/file"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/threading"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/snowflake"
	"google.golang.org/grpc"
)

// ServiceContext analytics 服务根装配（S8）。
type ServiceContext struct {
	Config config.Config
	Conn   sqlx.SqlConn
	Models *model.Models
	// Snowflake 导出任务 ID（读模型各表无业务写 ID 需求）。
	Snowflake  *snowflake.Node
	snowCancel func()
	// File 导出产物归档（未配置 = 导出渲染成功但归档失败记 error）。
	File fileclient.File
	// ExportRunner 导出渲染 worker 池（threading.TaskRunner：并发限流，
	// 与 RPC goroutine 隔离——FR-ANA-003 大导出对在线请求零影响）。
	ExportRunner *threading.TaskRunner
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	node, snowCancel, err := snowflake.NewAuto(context.Background(), c.Etcd.Hosts)
	if err != nil {
		panic(fmt.Errorf("snowflake 初始化失败: %w", err))
	}

	workers := c.ExportWorkers
	if workers < 1 {
		workers = 1
	}

	sc := &ServiceContext{
		Config:       c,
		Conn:         conn,
		Snowflake:    node,
		snowCancel:   snowCancel,
		Models:       model.NewModels(conn),
		ExportRunner: threading.NewTaskRunner(workers),
	}

	// FileRpc（缺失降级：导出可渲染但产物不归档，任务置 FAILED——功能明确报错优于静默）
	sc.File = mustFileClient(c.FileRpc)

	logx.Infof("analytics 就绪：db=micro_analytics file_rpc=%v export_workers=%d",
		sc.File != nil, workers)
	return sc
}

func mustFileClient(conf zrpc.RpcClientConf) fileclient.File {
	if len(conf.Etcd.Hosts) == 0 && conf.Target == "" {
		logx.Error("analytics: file RPC 未配置（导出产物无法归档，任务将置 FAILED）")
		return nil
	}
	cli := zrpc.MustNewClient(conf,
		zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor)),
		// 导出产物归档走 RPC bytes：消息上限 64MB（10 万行 CSV ~23MB）
		zrpc.WithDialOption(grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64<<20), grpc.MaxCallSendMsgSize(64<<20))))
	return fileclient.NewFile(cli)
}
