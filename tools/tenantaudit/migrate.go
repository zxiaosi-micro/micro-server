// migrate.go · 迁移 DDL 解析：CREATE TABLE 表名 + 是否含 tenant_id 列（R3 对账）。
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	createTableRe = regexp.MustCompile("(?i)create\\s+table\\s+(?:if\\s+not\\s+exists\\s+)?`?([\\w$]+)`?")
	tenantColRe   = regexp.MustCompile("(?i)`?tenant_id`?")
)

// tableDef 迁移中的建表定义。
type tableDef struct {
	Name      string
	HasTenant bool
}

// scanMigrations 解析服务 migrations/*.up.sql 全部建表（IF NOT EXISTS 幂等重跑安全）。
func scanMigrations(dir string) (map[string]tableDef, error) {
	out := map[string]tableDef{}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		parseDDL(string(raw), out)
	}
	return out, nil
}

// parseDDL 逐语句扫 CREATE TABLE 块：表名 + 列清单里有无 tenant_id。
func parseDDL(sql string, out map[string]tableDef) {
	for _, stmt := range splitStatements(sql) {
		m := createTableRe.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		td := tableDef{Name: strings.ToLower(m[1]), HasTenant: tenantColRe.MatchString(stmt)}
		if prev, ok := out[td.Name]; ok && prev.HasTenant {
			td.HasTenant = true // 后续迁移 add column tenant_id 也算
		}
		out[td.Name] = td
	}
}

// splitStatements 粗切分：按分号切（迁移 DDL 无存储过程，够用）。
func splitStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	inString := false
	for _, r := range sql {
		switch {
		case r == '\'':
			inString = !inString
			cur.WriteRune(r)
		case r == ';' && !inString:
			stmts = append(stmts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}
