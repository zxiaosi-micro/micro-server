package model

import (
	"sort"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrNotFound = sqlx.ErrNotFound

// buildSet map → "`a` = ?,`b` = ?" + 参数序（custom 局部更新共用，key 排序保证 SQL 稳定）。
func buildSet(fields map[string]any) (string, []any) {
	names := make([]string, 0, len(fields))
	for k := range fields {
		names = append(names, k)
	}
	sort.Strings(names)
	args := make([]any, 0, len(fields))
	sets := make([]string, 0, len(names))
	for _, k := range names {
		sets = append(sets, "`"+k+"` = ?")
		args = append(args, fields[k])
	}
	return strings.Join(sets, ", "), args
}
