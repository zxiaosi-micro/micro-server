// device 核心纯逻辑单测（S6-01）：状态机白名单/事件载荷/密钥生成。
package logic

import (
	"encoding/json"
	"testing"
)

// TestTransitionWhitelist FR-DEV-003：Transition 仅限白名单对。
func TestTransitionWhitelist(t *testing.T) {
	// RETIRED 只能从 ACTIVATED
	if allowed := transitionAllow["RETIRED"]; len(allowed) != 1 || allowed[0] != "ACTIVATED" {
		t.Fatalf("RETIRED 白名单异常: %v", allowed)
	}
	// SCRAPPED 允许从运行态任意迁入（IN_STOCK/OUT/ACTIVATED/RETIRED）
	seen := map[string]bool{}
	for _, from := range transitionAllow["SCRAPPED"] {
		seen[from] = true
	}
	for _, want := range []string{"IN_STOCK", "OUT", "ACTIVATED", "RETIRED"} {
		if !seen[want] {
			t.Fatalf("SCRAPPED 白名单缺 %s: %v", want, transitionAllow["SCRAPPED"])
		}
	}
	// PRODUCED 不可人工直改（状态机只由事件驱动）
	if _, ok := transitionAllow["IN_STOCK"]; ok {
		t.Fatal("IN_STOCK 不应允许人工迁移（stock.in 事件驱动）")
	}
}

// TestDeviceActivatedEventPayload contract 消费契约（warranty_kernel.go 解析 sn/activated_at）。
func TestDeviceActivatedEventPayload(t *testing.T) {
	ev := deviceActivatedEvent(1, "SN-1", 1234567890)
	if ev.Topic != "device_activated" {
		t.Fatalf("topic=%s", ev.Topic)
	}
	if ev.Type != "device.activated" {
		t.Fatalf("type=%s", ev.Type)
	}
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Sn          string `json:"sn"`
		ActivatedAt int64  `json:"activated_at"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.Sn != "SN-1" || p.ActivatedAt != 1234567890 {
		t.Fatalf("payload 不符: %s", raw)
	}
}

// TestGenDeviceSecret 32B base64url（FR-IOT-002 一机一密）。
func TestGenDeviceSecret(t *testing.T) {
	s, err := genDeviceSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 43 { // 32B → base64url 无 padding 43 字符
		t.Fatalf("secret 长度异常: %d", len(s))
	}
	s2, _ := genDeviceSecret()
	if s == s2 {
		t.Fatal("两次生成不应相同")
	}
}

// TestClampPage 分页收敛。
func TestClampPage(t *testing.T) {
	if p, s := clampPage(0, 0); p != 1 || s != 20 {
		t.Fatalf("默认分页异常: %d %d", p, s)
	}
	if p, s := clampPage(2, 500); p != 2 || s != 100 {
		t.Fatalf("上限收敛异常: %d %d", p, s)
	}
}
