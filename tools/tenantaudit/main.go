// tenantaudit · 多租户 SQL 规约静态对账（S11-01，ADR-08/16；CI 串行无依赖）。
//
// 规则：
//
//	R1 Exempt 登记必须携带理由（tenantx.Exempt(table, reason) reason 非空）
//	R2 Scoped 表的 custom model SQL：INSERT 列必含 tenant_id；
//	   SELECT/UPDATE/DELETE 必含 tenant_id 条件——豁免须行注释 `// tenantaudit:ignore <原因>`
//	   （cron 跨租户扫描/平台任务类）；*_gen.go 生成物不在审计面（custom 覆写 + 禁手改）
//	R3 迁移对账：建表含 tenant_id 列的业务表必须 Scoped 登记；Scoped 登记的表
//	   必须存在于该服务迁移且含 tenant_id 列。eventbus 四表（event_outbox/dedup/retry/dead）
//	   SQL 正本在 micro-common、无 tenant_id，内置豁免
//
// 用法（micro-server 根）：go run ./tools/tenantaudit [-dir services]
// 退出码：0 FAIL=0（WARN 允许）；1 存在 FAIL。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// eventbusSchemaTables 平台级事件四表：正本在 micro-common/eventbus/schema.go，无租户列。
var eventbusSchemaTables = map[string]bool{
	"event_outbox": true, "event_dedup": true, "event_retry": true, "event_dead": true,
}

var (
	failN int
	warnN int
	regs  = map[string]regEntry{} // table → 登记全局视图（多服务登记同名表以 scoped 优先）
)

func main() {
	dir := flag.String("dir", "services", "服务根目录")
	flag.Parse()

	svcDirs, err := filepath.Glob(filepath.Join(*dir, "*"))
	if err != nil || len(svcDirs) == 0 {
		fmt.Println("tenantaudit: 未发现服务目录（请在 micro-server 根运行）")
		os.Exit(1)
	}

	for _, svcDir := range svcDirs {
		fi, err := os.Stat(svcDir)
		if err != nil || !fi.IsDir() {
			continue
		}
		auditService(filepath.Base(svcDir), svcDir)
	}

	// 汇总登记视图（跨服务同名表：任一 scoped 即 scoped——共享表少见，保守取严）
	scopedTables := map[string]bool{}
	for _, r := range regs {
		if r.Kind == "scoped" {
			scopedTables[r.Table] = true
		}
	}

	fmt.Printf("\ntenantaudit: %d FAIL / %d WARN（登记 %d 表：scoped %d）\n",
		failN, warnN, len(regs), len(scopedTables))
	if failN > 0 {
		os.Exit(1)
	}
}

func fail(format string, a ...any) {
	failN++
	fmt.Printf("FAIL  "+format+"\n", a...)
}

func warn(format string, a ...any) {
	warnN++
	fmt.Printf("WARN  "+format+"\n", a...)
}

func auditService(svc, svcDir string) {
	modelDir := filepath.Join(svcDir, "internal", "model")
	files, _ := filepath.Glob(filepath.Join(modelDir, "*.go"))
	var scans []*fileScan
	for _, f := range files {
		if strings.HasSuffix(f, "_gen.go") || strings.HasSuffix(f, "_test.go") {
			continue
		}
		fs, err := scanModelFile(f, svc)
		if err != nil {
			warn("[%s] %s 解析失败: %v", svc, filepath.Base(f), err)
			continue
		}
		scans = append(scans, fs)
		for _, r := range fs.Regs {
			key := svc + ":" + r.Table
			if prev, ok := regs[key]; ok && prev.Kind != r.Kind {
				fail("[%s] %s:%d 表 %s 登记类型冲突（%s vs %s）", svc, filepath.Base(r.File), r.Line, r.Table, prev.Kind, r.Kind)
			}
			regs[key] = r
		}
	}

	// R1：Exempt 必须有理由
	for _, fs := range scans {
		for _, r := range fs.Regs {
			if r.Kind == "exempt" && strings.TrimSpace(r.Reason) == "" {
				fail("[R1][%s] %s:%d 表 %s Exempt 登记缺理由", svc, filepath.Base(r.File), r.Line, r.Table)
			}
		}
	}

	// R2：Scoped 表 SQL 判定
	scoped := map[string]bool{}
	for _, fs := range scans {
		for _, r := range fs.Regs {
			if r.Kind == "scoped" {
				scoped[r.Table] = true
			}
		}
	}
	for _, fs := range scans {
		for _, sql := range fs.SQL {
			auditSQL(svc, scoped, sql, fs)
		}
	}

	// R3：迁移对账（无迁移目录的服务跳过：iotingest 型）
	migDir := filepath.Join(svcDir, "migrations")
	if st, err := os.Stat(migDir); err == nil && st.IsDir() {
		tables, err := scanMigrations(migDir)
		if err != nil {
			warn("[%s] 迁移解析失败: %v", svc, err)
			return
		}
		registered := map[string]bool{}
		for _, fs := range scans {
			for _, r := range fs.Regs {
				registered[r.Table] = true
			}
		}
		var names []string
		for n := range tables {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			td := tables[n]
			if !td.HasTenant || eventbusSchemaTables[n] {
				continue
			}
			if !registered[n] {
				fail("[R3][%s] 迁移表 %s 含 tenant_id 列但未登记（应 tenantx.Scoped）", svc, n)
			}
		}
		for _, fs := range scans {
			for _, r := range fs.Regs {
				if r.Kind != "scoped" {
					continue
				}
				td, ok := tables[r.Table]
				if !ok {
					warn("[R3][%s] %s:%d Scoped 表 %s 不在本服务迁移（外部/共享表？）", svc, filepath.Base(r.File), r.Line, r.Table)
					continue
				}
				if !td.HasTenant {
					fail("[R3][%s] %s:%d Scoped 表 %s 迁移无 tenant_id 列", svc, filepath.Base(r.File), r.Line, r.Table)
				}
			}
		}
	}
}

var (
	reInsertInto = regexp.MustCompile("(?i)insert\\s+(?:ignore\\s+)?into\\s+`?([\\w$]+)`?")
	reFrom       = regexp.MustCompile("(?i)(?:from|join)\\s+`?([\\w$]+)`?")
	reUpdate     = regexp.MustCompile("(?i)update\\s+`?([\\w$]+)`?")
	reDelete     = regexp.MustCompile("(?i)delete\\s+from\\s+`?([\\w$]+)`?")
	reHasTenant  = regexp.MustCompile("(?i)`?tenant_id`?\\s*(=|,|\\)|$|\\s)")
	reWhereWord  = regexp.MustCompile(`(?i)\bwhere\b`)
)

// tenantCondOK 非 INSERT 语句的租户条件判定：tenant_id 必须出现在最后一个
// where 关键字之后（SELECT 列清单里的 tenant_id 列不算条件——orderRows 类
// 全列拼接不能成为漏判的遮蔽）。
func tenantCondOK(text string) bool {
	last := -1
	for _, loc := range reWhereWord.FindAllStringIndex(text, -1) {
		last = loc[1]
	}
	if last < 0 {
		return false
	}
	return reHasTenant.MatchString(text[last:])
}

// auditSQL 单条 SQL 判定：Scoped 表引用必须携带租户（INSERT 列 / WHERE 条件）。
func auditSQL(svc string, scoped map[string]bool, e sqlEntry, fs *fileScan) {
	if fs.isIgnored(e.Line) {
		return
	}
	text := e.Text

	check := func(table string, isInsert bool) {
		if !scoped[table] || eventbusSchemaTables[table] {
			return
		}
		if isInsert {
			if reHasTenant.MatchString(text) {
				return
			}
		} else if tenantCondOK(text) {
			return
		}
		verb := "条件"
		if isInsert {
			verb = "列"
		}
		fail("[R2][%s] %s:%d 表 %s 的 %s SQL 缺 tenant_id %s（跨租户扫描须加 // tenantaudit:ignore <原因>）｜%s",
			svc, filepath.Base(e.File), e.Line, table, firstVerb(text), verb, snippet(text))
	}

	if m := reInsertInto.FindStringSubmatch(text); m != nil {
		check(m[1], true)
		return
	}
	seen := map[string]bool{}
	if m := reUpdate.FindStringSubmatch(text); m != nil && !isUpdatePartOfInsert(text) {
		if !seen[m[1]] {
			seen[m[1]] = true
			check(m[1], false)
		}
	}
	if m := reDelete.FindStringSubmatch(text); m != nil {
		if !seen[m[1]] {
			seen[m[1]] = true
			check(m[1], false)
		}
	}
	for _, m := range reFrom.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			check(m[1], false)
		}
	}
}

// isUpdatePartOfInsert on duplicate key update 的 update 子句不重复计。
func isUpdatePartOfInsert(text string) bool {
	return strings.Contains(strings.ToLower(text), "on duplicate key update")
}

var reVerb = regexp.MustCompile(`(?i)\b(insert\s+into|delete\s+from|update|select)\b`)

func firstVerb(text string) string {
	if m := reVerb.FindStringSubmatch(text); m != nil {
		return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(m[1]), "  ", " "))
	}
	return "SQL"
}

// snippet SQL 片段（去多余空白，截 90 字符）用于 FAIL 输出甄别。
func snippet(text string) string {
	s := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(text), " ")
	if len(s) > 90 {
		s = s[:90] + "…"
	}
	return s
}

// isIgnored 豁免注释命中：SQL 起始行、上一行或同一赋值行的注释。
func (fs *fileScan) isIgnored(line int) bool {
	for l := line - 1; l <= line+1; l++ {
		if _, ok := fs.IgnoreAt[l]; ok {
			return true
		}
	}
	return false
}
