// 金额工具：DECIMAL 字符串 ↔ 分（int64）。
// 规约：金额运算一律走「分」的整数域（float 精度风险，02 §6.1 金额 DECIMAL 的实现口径）；
// 展示/传输仍是 DECIMAL 字符串。

package logic

import (
	"strings"
)

// parseCents "1299.50" → 129950；非法/超两位小数 → errAmountBad。
func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errAmountBad
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, fracPart = s[:i], s[i+1:]
	}
	if intPart == "" {
		intPart = "0"
	}
	if len(fracPart) > 2 {
		return 0, errAmountBad.WithMsg("金额小数位超过 2 位")
	}
	var cents int64
	for _, c := range []byte(intPart) {
		if c < '0' || c > '9' {
			return 0, errAmountBad
		}
		cents = cents*10 + int64(c-'0')
	}
	cents = cents*100 + int64(fracByte(fracPart, 0))*10 + int64(fracByte(fracPart, 1))
	if neg {
		cents = -cents
	}
	return cents, nil
}

func fracByte(frac string, i int) byte {
	if i < len(frac) {
		if frac[i] >= '0' && frac[i] <= '9' {
			return frac[i] - '0'
		}
	}
	return 0
}

// centsToAmount 129950 → "1299.50"。
func centsToAmount(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return sign + itoa(cents/100) + "." + pad2(cents % 100)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func pad2(v int64) string {
	if v < 10 {
		return "0" + itoa(v)
	}
	return itoa(v)
}
