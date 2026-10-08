// 逻辑层共享小工具（order/finance 同款约定）。

package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// sql.Null* 便捷构造。
func sqlInt64(v int64) sql.NullInt64   { return sql.NullInt64{Int64: v, Valid: v != 0} }
func sqlTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: !t.IsZero()} }
func sqlString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func floatToCents(f float64) int64 {
	return int64(math.Round(f * 100))
}

func centsToAmount(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return sign + itoa(cents/100) + "." + pad2(cents%100)
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

func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errAmountBad
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
	return cents*100 + int64(fracByte(fracPart, 0))*10 + int64(fracByte(fracPart, 1)), nil
}

func fracByte(frac string, i int) byte {
	if i < len(frac) {
		if frac[i] >= '0' && frac[i] <= '9' {
			return frac[i] - '0'
		}
	}
	return 0
}

// tenantSkip 后台事件消费显式路径（信封已带租户；扫描类用）。
func tenantSkip(ctx context.Context) context.Context {
	return tenantx.Skip(ctx)
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
