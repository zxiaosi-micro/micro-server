// warehouse 表 custom 覆写（ADR-08）：租户过滤 + NoCache 查询（口径同 party/catalog）。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ WarehouseModel = (*customWarehouseModel)(nil)

type (
	// WarehouseModel 仓库模型。
	WarehouseModel interface {
		// Insert 新建仓库（生成方法；data.TenantId 必填；code 租户内 UK）。
		Insert(ctx context.Context, data *Warehouse) (sql.Result, error)
		// FindPage 仓库列表（keyword 模糊 name/code）。
		FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*Warehouse, int64, error)
	}

	customWarehouseModel struct {
		*defaultWarehouseModel
	}
)

// NewWarehouseModel returns a model for the database table.
func NewWarehouseModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) WarehouseModel {
	return &customWarehouseModel{
		defaultWarehouseModel: newWarehouseModel(conn, c, opts...),
	}
}

func (m *customWarehouseModel) FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*Warehouse, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`name` like ? or `code` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `warehouse_id` desc limit ? offset ?",
		warehouseRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Warehouse
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
