// 退役评估评级判定单测（S10-01 验收：三评级分支各一例）。
package logic

import "testing"

func TestRetireGradeBranches(t *testing.T) {
	cases := []struct {
		name          string
		soh           float64
		cycles        int64
		consistencyOk bool
		safetyPass    bool
		want          string
	}{
		{"A_SOH80循环1499一致达标", 80, 1499, true, true, "A"},
		{"B_SOH75", 75, 100, true, true, "B"},
		{"C_SOH低于70", 65, 100, true, true, "C"},
		{"B_循环超限降容", 90, 1500, true, true, "B"},
		{"B_一致不达标降容", 90, 100, false, true, "B"},
		{"C_安全不过", 95, 10, true, false, "C"},
		{"B_SOH边界70", 70, 100, true, true, "B"},
		{"C_SOH边界69.99", 69.99, 100, true, true, "C"},
		{"A_循环边界1499", 85, 1499, true, true, "A"},
		{"B_循环边界1500", 85, 1500, true, true, "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grade, reason := retireGrade(c.soh, c.cycles, c.consistencyOk, c.safetyPass)
			if grade != c.want {
				t.Fatalf("grade=%s want=%s (reason=%s)", grade, c.want, reason)
			}
		})
	}
}
