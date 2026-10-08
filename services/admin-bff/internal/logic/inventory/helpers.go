// S4 业务域转发公共助手（S4-05）：ID 解析、JSON 数组解析、业务类型兜底。

package inventory

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

// parseID 字符串 ID → int64（E8：前端一律字符串传输；空/非法 → 0）。
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

// opBizType 库存操作业务类型兜底（管理端手动操作统一 ADMIN 语义）。
func opBizType(t string) string {
	if t == "" {
		return "ADMIN"
	}
	return t
}

var (
	_ = strconv.Itoa
	_ = time.Now
	_ = ctxkit.UID
	_ = json.Valid
)
