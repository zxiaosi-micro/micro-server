package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"micro-server/services/hello/internal/confcenter"
	"micro-server/services/hello/internal/config"
	"micro-server/services/hello/internal/confx"
	"micro-server/services/hello/internal/cronx"
	"micro-server/services/hello/internal/handler"
	"micro-server/services/hello/internal/svc"

	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zxiaosi-micro/micro-common/response"
)

var configFile = flag.String("f", "etc/hello.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// 统一信封（02 §3.3）——所有服务 main 必调
	response.Setup()

	// configcenter listener（S3-06 示例：运行时参数热更新；连不上 etcd 用默认值）
	confcenter.MustListen(c.Etcd.Hosts, c.ConfigKey)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	// cron 注册表（S3-06 示例：任务登记 + service.Service 挂载；引用 svcCtx 的连接）
	registerCron(ctx)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}

// registerCron cron 注册表示例：新服务照抄（登记 → cronx.NewService 挂 ServiceGroup）。
// 纪律（ADR-09）：内存态定时只做加速，跨重启的兜底走业务表 next_exec_at 扫描。
func registerCron(sc *svc.ServiceContext) {
	cronx.Register(cronx.Task{
		Name:     "hello-heartbeat",
		Interval: 30 * time.Second,
		Run: func(ctx context.Context) error {
			// 示例任务：探活 DB（生产任务替换为业务扫描，如 cmd 表重发、工单升级）
			var n int64
			return sc.Conn.QueryRowCtx(ctx, &n, "select count(*) from `greeting` where `deleted_at` is null")
		},
	})
	group := service.ServiceGroup{}
	group.Add(cronx.NewService())
	group.Start()
}
