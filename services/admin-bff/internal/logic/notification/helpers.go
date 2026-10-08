// S4 业务域转发公共助手（S4-05）。

package notification

import "strconv"

// parseID 字符串 ID → int64（E8）。
func parseID(s string) int64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
