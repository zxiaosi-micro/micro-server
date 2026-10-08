// 内部公共助手：上下文取值、空值转换、金额解析、分页收敛。

package logic

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"

	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

// opUID 操作人 uid（BFF authz 注入；后台作业为 0）。
func opUID(ctx context.Context) int64 {
	return ctxkit.UID(ctx)
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

// parseAmount 金额字符串 → DECIMAL（两位小数四舍五入；空串非法——价格必填）。
func parseAmount(s string) (float64, error) {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, err
	}
	return math.Round(f*100) / 100, nil
}

// formatAmount DECIMAL → 字符串（两位小数）。
func formatAmount(f float64) string {
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', 2, 64)
}

// clampPage 分页上限收敛（size 上限 100，与前端 usePaged 同源约束）。
func clampPage(page, size int64) (int64, int64) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
