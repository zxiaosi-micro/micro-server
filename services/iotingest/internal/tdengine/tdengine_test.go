// tdengine 命名单测（S6-03，ADR-13）：超级表/子表名合法化。
package tdengine

import "testing"

func TestSanitize(t *testing.T) {
	if got := sanitize("ESS-DEMO"); got != "ess_demo" {
		t.Fatalf("sanitize=%s", got)
	}
	if got := sanitize("SN.001/X"); got != "sn_001_x" {
		t.Fatalf("sanitize=%s", got)
	}
	if got := stableName("ESS-DEMO"); got != "iot_ess_demo" {
		t.Fatalf("stableName=%s", got)
	}
	if got := tableName("ESS-DEMO", "SN-001"); got != "iot_ess_demo_sn_001" {
		t.Fatalf("tableName=%s", got)
	}
}
