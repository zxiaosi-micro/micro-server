// S4 业务域转发公共助手（S4-05）。

package party

import (
	"encoding/json"
	"strconv"
)

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

// parseStringSlice JSON 数组字符串 → []string（party.type/skill_tags）。
func parseStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
