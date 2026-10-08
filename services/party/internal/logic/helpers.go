// 内部公共助手：上下文取值、空值转换、索引哈希、JSON 序列化。

package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

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

func toNullTime(millis int64) sql.NullTime {
	if millis <= 0 {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: time.UnixMilli(millis), Valid: true}
}

func nullTimeMilli(t sql.NullTime) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.UnixMilli()
}

// hmacHex HMAC-SHA256 索引哈希（contact.mobile_hash；与 identity/seed 口径一致）。
func hmacHex(key []byte, plain string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(plain))
	return hex.EncodeToString(mac.Sum(nil))
}

// marshalJSON JSON 序列化（失败不该发生——入参已是合法结构；兜底空串）。
func marshalJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

// validJSONString 校验入参字符串是合法 JSON（notify_pref/rebate_rule/skill_tags 类字段）。
func validJSONString(s string) bool {
	if s == "" {
		return true
	}
	return json.Valid([]byte(s))
}
