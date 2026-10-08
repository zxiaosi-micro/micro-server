package model

import (
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrNotFound = sqlx.ErrNotFound
	// ErrStatusConflict 状态 CAS 转移冲突。
	ErrStatusConflict = errors.New("model: 状态冲突(已被并发转移)")
)
