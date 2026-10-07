package org

import "strconv"

// parseID string ID → int64（E8：对外一律字符串，BFF 层转换）。
func parseID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
