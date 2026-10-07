package menu

import "strconv"

// parseID string ID → int64（E8）。
func parseID(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
