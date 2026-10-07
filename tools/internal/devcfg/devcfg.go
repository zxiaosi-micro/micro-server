// Package devcfg 是 S2 工具链共享的 dev 环境配置读取。
//
// 读取优先级：进程环境变量 > micro-deploy/compose/dev/.env > 内置默认值。
// .env 回读让工具在"从 micro-server 根目录 go run"场景下零配置可用
// （compose 已用 .env 拉起中间件，工具读同一份凭据，无需 shell export）。
package devcfg

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadResult 记录 .env 回读来源，便于工具输出诊断信息。
type LoadResult struct {
	Path  string // 实际读到的 .env 路径（空 = 未找到，全靠环境变量/默认值）
	Count int    // 回读的键数量
}

var loaded LoadResult

// Env 返回单例加载结果（首次访问时完成 .env 探测与解析）。
func Env() LoadResult {
	if loaded.Count == 0 && loaded.Path == "" {
		loaded = load()
	}
	return loaded
}

// Get 按优先级取值；env 优先，其次 .env，最后 fallback（可为空串）。
func Get(key, fallback string) string {
	Env()
	if v := os.Getenv(key); v != "" {
		return v
	}
	if v, ok := dotEnv[key]; ok && v != "" {
		return v
	}
	return fallback
}

var dotEnv = map[string]string{}

// load 依次探测候选 .env：micro-server 根 / tools 目录 / 仓库根 三种执行位置。
func load() LoadResult {
	candidates := []string{
		"../micro-deploy/compose/dev/.env",       // 在 micro-server 根执行
		"../../micro-deploy/compose/dev/.env",    // 在 micro-server/tools 子目录执行
		"../../../micro-deploy/compose/dev/.env", // 更深层级（go run ./tools/xxx 时 CWD=模块根）
	}
	for _, p := range candidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if m, n := parseDotEnv(abs); n > 0 {
			dotEnv = m
			return LoadResult{Path: abs, Count: n}
		}
	}
	return LoadResult{}
}

// parseDotEnv 解析 KEY=VALUE 行（跳过注释/空行；值可带引号）。
func parseDotEnv(path string) (map[string]string, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer f.Close()

	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
			v = v[1 : len(v)-1]
		}
		m[k] = v
	}
	return m, len(m)
}
