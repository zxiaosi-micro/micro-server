// iotingest 核心纯逻辑单测（S6-03）：MQTT 主题解析/规则判定/超级表命名。
package forwarder

import (
	"testing"
)

// TestParseTopic 共享订阅主题契约：up/{tenant}/{pk}/{sn}/telemetry（02 §7.2 ACL 最小主题）。
func TestParseTopic(t *testing.T) {
	tenant, pk, sn, err := parseTopic("up/1/ESS-DEMO/DEV-001/telemetry")
	if err != nil || tenant != 1 || pk != "ESS-DEMO" || sn != "DEV-001" {
		t.Fatalf("合法主题解析异常: %v %d %s %s", err, tenant, pk, sn)
	}
	// 非 telemetry 后缀（cmd_ack 走独立转发，不在本 filter）
	if _, _, _, err := parseTopic("up/1/PK/SN/cmd_ack"); err == nil {
		t.Fatal("cmd_ack 主题不应命中 telemetry filter")
	}
	// 段数不符
	if _, _, _, err := parseTopic("up/1/PK/SN"); err == nil {
		t.Fatal("缺段主题应拒绝")
	}
	// tenant 非数字
	if _, _, _, err := parseTopic("up/x/PK/SN/telemetry"); err == nil {
		t.Fatal("非数字 tenant 应拒绝")
	}
}
