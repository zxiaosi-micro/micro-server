package model

import (
	"database/sql"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrNotFound = sqlx.ErrNotFound

// toNullInt64 模型层通用空值转换（updated_by 类审计列）。
func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}
