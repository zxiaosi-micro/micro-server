// ops 纯函数单测（S7-01：cron 展开/状态机/技能匹配）。
package logic

import (
	"testing"
	"time"
)

func TestNextExecAt(t *testing.T) {
	from := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		expr string
		want time.Time
		ok   bool
	}{
		{"@daily", time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC), true},
		{"@weekly", time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC), true},
		{"@monthly", time.Date(2026, 11, 11, 8, 0, 0, 0, time.UTC), true},
		{"@h:6", from.Add(6 * time.Hour), true},
		{"0 8 * * *", time.Time{}, false}, // 完整 cron 不支持（简化集合）
		{"@h:0", time.Time{}, false},      // 非法间隔
		{"@h:999", time.Time{}, false},    // 超上限
	}
	for _, c := range cases {
		got, err := nextExecAt(from, c.expr)
		if c.ok && (err != nil || !got.Equal(c.want)) {
			t.Errorf("nextExecAt(%q) = %v, %v; want %v", c.expr, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("nextExecAt(%q) 应报 errInspectCronBad", c.expr)
		}
	}
}

func TestTransitionAllowed(t *testing.T) {
	// 主链路全通
	chain := []string{"PENDING_DISPATCH", "DISPATCHED", "ACCEPTED", "PROCESSING", "SUBMITTED", "VERIFIED", "CLOSED"}
	for i := 0; i < len(chain)-1; i++ {
		if !transitionAllowed(chain[i], chain[i+1]) {
			t.Errorf("主链路迁移 %s→%s 应允许", chain[i], chain[i+1])
		}
	}
	// 非法跳变
	if transitionAllowed("PENDING_DISPATCH", "ACCEPTED") {
		t.Error("待派直接受应拒绝")
	}
	if transitionAllowed("CLOSED", "OPEN") {
		t.Error("终态无出边")
	}
	// 验收退回
	if !transitionAllowed("SUBMITTED", "PROCESSING") {
		t.Error("验收退回应允许")
	}
	// 任意非终态可取消
	for _, s := range []string{"PENDING_DISPATCH", "DISPATCHED", "ACCEPTED", "PROCESSING", "SUBMITTED"} {
		if !transitionAllowed(s, "CANCELLED") {
			t.Errorf("%s 应可取消", s)
		}
	}
}

func TestSkillsCover(t *testing.T) {
	if !skillsCover(`["电池","逆变器"]`, []string{"电池"}) {
		t.Error("技能子集应命中")
	}
	if skillsCover(`["电池"]`, []string{"电池", "逆变器"}) {
		t.Error("技能不足不应命中")
	}
	if skillsCover("", []string{"电池"}) {
		t.Error("空技能不应命中")
	}
	if skillsCover(`not-json`, []string{"电池"}) {
		t.Error("坏 JSON 不应命中")
	}
	if !skillsCover(`["x"]`, nil) {
		t.Error("无技能要求应全员命中")
	}
}

func TestAppendDeviceJSON(t *testing.T) {
	first := appendDeviceJSON("", "SN001", 11)
	if first != `[{"sn":"SN001","device_id":11}]` {
		t.Errorf("首设备明细 = %s", first)
	}
	two := appendDeviceJSON(first, "SN002", 0)
	want := `[{"sn":"SN001","device_id":11},{"sn":"SN002"}]`
	if two != want {
		t.Errorf("聚合明细 = %s; want %s", two, want)
	}
}
