// configpush · 配置正本校验 + 推送 etcd（S2-06，ADR-10）
//
// 正本目录：micro-deploy/config/<svc>/<file>.json|yml|yaml（按服务分文件）
// etcd 键位：/micro/config/<svc>/<file 去扩展名>（与 configcenter 的 exact-key watch 对应）
//
// 链路口径（E15 排障顺序：先查推送是否成功，再查服务 listener）：
//
//	改 Git → PR 合并 → configpush 推送 → etcd 更新 → 服务 ConfigCenter listener 秒级 reload
//
// 用法：
//
//	go run ./tools/configpush -validate                 # 只做结构校验（CI PR 门槛用，无需 etcd）
//	go run ./tools/configpush                           # 校验 + 全量推送（值未变化的键自动跳过）
//	go run ./tools/configpush -file ops/alert_baseline.yml -dry-run
//	go run ./tools/configpush -demo                     # listener 演示（demo.go，验证 3s 内 reload）
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"

	"micro-server/tools/internal/devcfg"
)

const defaultPrefix = "/micro/config"

const dialTimeout = 3 * time.Second

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

type fileItem struct {
	svc  string // 服务段（_platform = 平台全局）
	name string // 文件名去扩展名
	path string
	raw  []byte
}

func main() {
	dir := flag.String("dir", "../micro-deploy/config", "配置正本目录")
	file := flag.String("file", "", "只处理相对 <dir> 的单个文件（如 ops/polling.yml）")
	validate := flag.Bool("validate", false, "只校验不推送（CI 用，无需 etcd）")
	dryRun := flag.Bool("dry-run", false, "推送演练：不写 etcd")
	prefixFlag := flag.String("prefix", "", "etcd 键前缀（默认 /micro/config，或 CONFIG_KEY_PREFIX）")
	etcdFlag := flag.String("etcd", "", "etcd 地址（默认 127.0.0.1:22379，或 MICRO_DEV_ETCD_HOSTS）")
	demo := flag.Bool("demo", false, "启动 listener 演示")
	demoKey := flag.String("demo-key", "_demo/app", "演示监听的 <dir> 相对键")
	flag.Parse()

	if *demo {
		runDemo(*dir, *demoKey, *etcdFlag)
		return
	}

	items, err := collect(*dir, *file)
	if err != nil {
		fatal(err)
	}
	if len(items) == 0 {
		fmt.Printf("configpush: %s 下无配置文件（期待 <svc>/<file>.json|yml|yaml）\n", *dir)
		return
	}

	fmt.Printf("== configpush · 校验 %d 个配置文件（%s）==\n", len(items), *dir)
	for _, it := range items {
		fmt.Printf("  %-40s ✓（%dB）\n", it.svc+"/"+it.name, len(it.raw))
	}

	if *validate {
		fmt.Println("== 校验通过（-validate 模式，未推送）==")
		return
	}

	prefix := *prefixFlag
	if prefix == "" {
		prefix = devcfg.Get("CONFIG_KEY_PREFIX", defaultPrefix)
	}
	endpoint := *etcdFlag
	if endpoint == "" {
		endpoint = devcfg.Get("MICRO_DEV_ETCD_HOSTS", "127.0.0.1:22379")
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}, DialTimeout: dialTimeout})
	if err != nil {
		fatal(fmt.Errorf("连 etcd %s: %w（先跑 envcheck）", endpoint, err))
	}
	defer cli.Close()
	ctx, cancel := ctxTimeout()
	defer cancel()

	fmt.Printf("== 推送 → %s @ %s ==\n", prefix, endpoint)
	fail := 0
	for _, it := range items {
		key := fmt.Sprintf("%s/%s/%s", prefix, it.svc, it.name)
		if *dryRun {
			fmt.Printf("  [dry-run] %-46s 将推送 %dB\n", key, len(it.raw))
			continue
		}
		existing, err := cli.Get(ctx, key)
		if err != nil {
			fmt.Printf("  %-46s ✗ Get: %v\n", key, err)
			fail++
			continue
		}
		if len(existing.Kvs) == 1 && string(existing.Kvs[0].Value) == string(it.raw) {
			fmt.Printf("  %-46s - 未变化，跳过（不触发 listener）\n", key)
			continue
		}
		if _, err := cli.Put(ctx, key, string(it.raw)); err != nil {
			fmt.Printf("  %-46s ✗ Put: %v\n", key, err)
			fail++
			continue
		}
		fmt.Printf("  %-46s ✓ 已推送（listener 将秒级 reload）\n", key)
	}
	if fail > 0 {
		fmt.Printf("== 失败 %d 项 ==\n", fail)
		os.Exit(1)
	}
	fmt.Println("== 推送完成 ==")
}

// collect 扫描 <dir>/<svc>/<file>.(json|yaml|yml) 并做结构校验：
// json 必须是 JSON 对象，yaml 必须是映射；空文件/嵌套层级超一概拒绝。
func collect(root, only string) ([]fileItem, error) {
	var items []fileItem
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("读正本目录 %s: %w", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		svc := e.Name()
		files, err := os.ReadDir(filepath.Join(root, svc))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				return nil, fmt.Errorf("%s/%s: 只允许一层（按服务分文件），不允许子目录", svc, f.Name())
			}
			ext := strings.ToLower(filepath.Ext(f.Name()))
			if ext != ".json" && ext != ".yaml" && ext != ".yml" {
				continue
			}
			if only != "" && fmt.Sprintf("%s/%s", svc, f.Name()) != only && fmt.Sprintf("%s/%s", svc, strings.TrimSuffix(f.Name(), ext)) != only {
				continue
			}
			it := fileItem{svc: svc, name: strings.TrimSuffix(f.Name(), ext)}
			it.path = filepath.Join(root, svc, f.Name())
			raw, err := os.ReadFile(it.path)
			if err != nil {
				return nil, err
			}
			if len(strings.TrimSpace(string(raw))) == 0 {
				return nil, fmt.Errorf("%s/%s: 空文件", svc, f.Name())
			}
			it.raw = raw
			if err := validateStruct(ext, raw); err != nil {
				return nil, fmt.Errorf("%s/%s: %w", svc, f.Name(), err)
			}
			items = append(items, it)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].svc != items[j].svc {
			return items[i].svc < items[j].svc
		}
		return items[i].name < items[j].name
	})
	return items, nil
}

func validateStruct(ext string, raw []byte) error {
	switch ext {
	case ".json":
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("JSON 解析失败: %w", err)
		}
		if m == nil {
			return fmt.Errorf("顶层必须是 JSON 对象")
		}
	default:
		var m map[string]any
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("YAML 解析失败: %w", err)
		}
		if m == nil {
			return fmt.Errorf("顶层必须是 YAML 映射")
		}
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "configpush: %v\n", err)
	os.Exit(1)
}
