// deps.go · logic 层共享依赖与工具（读模型/导出/对账共用）。

package logic

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"micro-server/services/analytics/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// readDeps 读模型消费内核依赖（避免循环依赖：consumer 包 → logic 内核）。
type readDeps struct {
	conn      sqlx.SqlConn
	models    *model.Models
	readGroup string // 消费组（含版本号，watermark 记录用）
}

var (
	errDupBiz = errors.New("analytics: 业务键重复")
)

// isDupKey MySQL 1062 判定（台账 uk_biz 幂等出口）。
// go-zero sqlx 透传 go-sql-driver/mysql 错误，串判定避免引入驱动类型依赖。
func isDupKey(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}

// parseDay "yyyy-MM-dd" → 本地零点。
func parseDay(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
