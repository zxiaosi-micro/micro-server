// helpers.go · app 包共用转换。

package app

import "strconv"

func i64s(n int64) string { return strconv.FormatInt(n, 10) }

// parseI64 端侧 string ID → int64（非法=0，下游 ErrNotFound 语义统一）。
func parseI64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
