// 逻辑层共享小工具（与 order 服务同款约定）。

package logic

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// sql.Null* 便捷构造。
func sqlInt64(v int64) sql.NullInt64   { return sql.NullInt64{Int64: v, Valid: v != 0} }
func sqlTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: !t.IsZero()} }
func sqlString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func sqlFloat(cents int64) sql.NullFloat64 {
	if cents == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: float64(cents) / 100, Valid: true}
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

// tenantSkip 后台作业显式豁免租户（tools/reconcile 日结）。
func tenantSkip(ctx context.Context) context.Context {
	return tenantx.Skip(ctx)
}
