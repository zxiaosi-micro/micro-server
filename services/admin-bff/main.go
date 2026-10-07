package main

import (
	"flag"
	"fmt"
	"strings"

	"micro-server/services/admin-bff/internal/config"
	"micro-server/services/admin-bff/internal/confx"
	"micro-server/services/admin-bff/internal/handler"
	"micro-server/services/admin-bff/internal/svc"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zxiaosi-micro/micro-common/response"
)

var configFile = flag.String("f", "etc/admin-bff.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	confx.MustLoad(*configFile, &c)

	// 统一信封 {code,msg,data} + 认证码 401 / 权限 403 / 限流 429（02 §3.3）
	response.Setup()
	// goctl httpx.Parse 的 options/range 校验错误无公开哨兵——按错误语义登记归一 110400
	response.SetBadRequestClassifier(func(err error) bool {
		msg := err.Error()
		return strings.Contains(msg, "not defined in options") ||
			strings.Contains(msg, "invalid range") ||
			strings.Contains(msg, "not in range")
	})

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	server.Use(svc.BearerProbe(ctx.Verifier)) // 免鉴权组（logout/step-up）注入 uid/sid
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
