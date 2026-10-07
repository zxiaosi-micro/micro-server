// demo.go · configcenter listener 演示（S2-06，ADR-10）
//
// 验证"改 Git → configpush → etcd 更新 → 服务 3s 内 reload"全链路（排障口径 E15）：
//
//	终端 A：go run ./tools/configpush -demo
//	终端 B：编辑 ../micro-deploy/config/_demo/app.yml（改 poll_interval_sec）
//	        → go run ./tools/configpush -file _demo/app.yml
//	终端 A 输出 reload 与耗时；ConfigCenter[T] 泛型 + AddListener 回调即服务侧消费形态。
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	configcenter "github.com/zeromicro/go-zero/core/configcenter"
	"github.com/zeromicro/go-zero/core/configcenter/subscriber"

	"micro-server/tools/internal/devcfg"
)

// demoConf 演示用强类型配置（服务侧即此形态：结构体 + go-zero 标签校验）。
type demoConf struct {
	PollIntervalSec int  `json:"poll_interval_sec,range=[1:60]"`
	FeatureEnabled  bool `json:"feature_enabled"`
}

func runDemo(dir, relKey, etcdFlag string) {
	endpoint := etcdFlag
	if endpoint == "" {
		endpoint = devcfg.Get("MICRO_DEV_ETCD_HOSTS", "127.0.0.1:22379")
	}
	key := "/micro/config/" + relKey

	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relKey)+".yml")); err != nil {
		fmt.Fprintf(os.Stderr, "configpush -demo: 演示配置 %s/%s.yml 不存在\n", dir, relKey)
		os.Exit(1)
	}

	// ---- 核心演示（约 20 行）：ConfigCenter[T] + AddListener ----
	cc := configcenter.MustNewConfigCenter[demoConf](
		configcenter.Config{Type: "yaml"},
		subscriber.MustNewEtcdSubscriber(subscriber.EtcdConf{Hosts: []string{endpoint}, Key: key}),
	)
	start, _ := cc.GetConfig()
	fmt.Printf("[demo] etcd=%s key=%s\n[demo] 初始配置: %+v\n", endpoint, key, start)
	cc.AddListener(func() {
		c, err := cc.GetConfig()
		if err != nil {
			fmt.Printf("[demo] reload 失败: %v\n", err)
			return
		}
		fmt.Printf("[demo] %s 配置变更 → listener reload 完成: %+v\n", time.Now().Format("15:04:05.000"), c)
	})
	// ---- 演示核心结束 ----

	fmt.Println("[demo] 等待变更中……编辑 _demo/app.yml 后执行: go run ./tools/configpush -file _demo/app.yml")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\n[demo] 退出")
}
