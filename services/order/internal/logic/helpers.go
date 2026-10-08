// 逻辑层共享小工具。

package logic

import (
	"context"
	"database/sql"
	"math"
	"time"

	"micro-server/services/order/internal/confcenter"
	"micro-server/services/order/internal/svc"

	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// tenantxWith / tenantxSkipCtx 事件消费与扫描任务的租户上下文（信封恢复/显式豁免）。
func tenantxWith(ctx context.Context, tid int64) context.Context {
	return tenantx.WithTenant(ctx, tid)
}

func tenantxSkipCtx(ctx context.Context) context.Context {
	return tenantx.Skip(ctx)
}

// floatToCents DB DECIMAL（float64 承载）→ 分（四舍五入规避二进制浮点截断）。
func floatToCents(f float64) int64 {
	return int64(math.Round(f * 100))
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// advanceBatchOf 扫描批量（confcenter 热调，默认 100）。
func advanceBatchOf(sc *svc.ServiceContext) int {
	if b := confcenter.Current().AdvanceBatch; b > 0 {
		return b
	}
	return 100
}

// effectivePayTimeoutMin 生效支付超时分钟数（confcenter 覆盖 > 启动配置 > 默认 30，FR-ORD-006）。
func effectivePayTimeoutMin(sc *svc.ServiceContext) int {
	if v := confcenter.Current().PayTimeoutMinutes; v > 0 {
		return v
	}
	if v := sc.Config.PayTimeoutMinutes; v > 0 {
		return v
	}
	return 30
}

// ---- sql.Null* 便捷构造（Logic 层填模型用）----

func sqlInt64(v int64) sql.NullInt64  { return sql.NullInt64{Int64: v, Valid: v != 0} }
func sqlTime(t time.Time) sql.NullTime { return sql.NullTime{Time: t, Valid: !t.IsZero()} }
func sqlString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func uidAsNull(uid int64) sql.NullInt64 { return sqlInt64(uid) }

// errDownMiss 下游未配置的稳定业务错误（可重试失败的语义由 Saga 层承担，此处面向同步调用面）。
func errDownMiss() error {
	return errDownstreamMiss
}
