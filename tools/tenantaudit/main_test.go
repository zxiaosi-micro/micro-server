// tenantaudit 自测：合成微型服务（model + 迁移）验证三类规则与豁免注释（S11-01）。
// 夹具 SQL 不用反引号（与 Go 原生字符串定界冲突；扫描器对两种形式等价处理）。
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditRules(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "fixtures/internal/model/registry.go", `package model
import "github.com/zxiaosi-micro/micro-common/tenantx"
func init() {
	tenantx.Scoped("order")
	tenantx.Exempt("tenant", "租户根表：平台面管理")
}
`)
	// orderRow 含 tenant_id 列（模拟 gen rows）；FindPayExpired 跨租户扫描带豁免注释。
	write(t, dir, "fixtures/internal/model/orders.go", `package model
var orderRow = "order_id,tenant_id"

func (m *customOrderModel) ListPage(where string) string {
	return "select " + orderRow + " from order where " + where
}

func (m *customOrderModel) Insert(data *Order) error {
	_, err := m.conn.Exec("insert into order (order_no) values (?)")
	return err
}

func (m *customOrderModel) FindPayExpired(now string) string {
	// tenantaudit:ignore 支付超时 cron 跨租户扫描
	return "select " + orderRow + " from order where pay_expire_at <= ?"
}
`)
	write(t, dir, "fixtures/migrations/000001_init.up.sql", "CREATE TABLE IF NOT EXISTS `order` (\n  `id` BIGINT,\n  `tenant_id` BIGINT NOT NULL\n);\nCREATE TABLE IF NOT EXISTS `orphan` (\n  `id` BIGINT,\n  `tenant_id` BIGINT NOT NULL\n);\n")

	failN, warnN = 0, 0
	regs = map[string]regEntry{}
	auditService("fixtures", filepath.Join(dir, "fixtures"))

	// 预期 FAIL：
	//  R3 orphan 表含 tenant_id 未登记；R2 Insert 缺 tenant_id 列；
	//  R2 ListPage 的 where 形参不可求值（⟨where⟩ 占位）→ 判定缺 tenant_id 条件。
	if failN != 3 {
		t.Fatalf("预期 3 FAIL，实得 %d（warn %d）", failN, warnN)
	}
}

func TestIgnoreWindow(t *testing.T) {
	fs := &fileScan{IgnoreAt: map[int]string{10: "x"}}
	for _, l := range []int{9, 10, 11} {
		if !fs.isIgnored(l) {
			t.Fatalf("行 %d 应命中豁免（±1 窗口）", l)
		}
	}
	if fs.isIgnored(12) {
		t.Fatal("行 12 不应命中豁免")
	}
}
