// rules 判定单测（S6-03，FR-IOT-009）：比较器命中语义。
package rules

import "testing"

func TestHit(t *testing.T) {
	cases := []struct {
		cmp   string
		v, th float64
		want  bool
	}{
		{"gt", 55.1, 55, true},
		{"gt", 55, 55, false},
		{"gte", 55, 55, true},
		{"lt", 9.9, 10, true},
		{"lt", 10, 10, false},
		{"lte", 10, 10, true},
		{"unknown", 100, 10, false}, // 未知比较器不命中（安全默认）
	}
	for _, c := range cases {
		if got := hit(c.cmp, c.v, c.th); got != c.want {
			t.Fatalf("hit(%s,%v,%v)=%v want %v", c.cmp, c.v, c.th, got, c.want)
		}
	}
}
