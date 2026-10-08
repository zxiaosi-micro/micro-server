// stocktake 表 custom 覆写（ADR-08）：盘点单 DRAFT(1)→SUBMITTED(2)→APPROVED(3)/REJECTED(4)。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ StocktakeModel = (*customStocktakeModel)(nil)

type (
	// StocktakeModel 盘点单模型。
	StocktakeModel interface {
		// Insert 新建盘点单（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *Stocktake) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, stocktakeId int64) (*Stocktake, error)
		// FindPage 盘点单列表（wid 0=全部；status 0=全部）。
		FindPage(ctx context.Context, tenantId, warehouseId int64, status int64, page, size int64) ([]*Stocktake, int64, error)
		// UpdateStatus 状态流转（提交/审批/驳回）。
		UpdateStatus(ctx context.Context, tenantId, stocktakeId, status int64, remark string, updatedBy int64) error
	}

	customStocktakeModel struct {
		*defaultStocktakeModel
	}
)

// NewStocktakeModel returns a model for the database table.
func NewStocktakeModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StocktakeModel {
	return &customStocktakeModel{
		defaultStocktakeModel: newStocktakeModel(conn, c, opts...),
	}
}

func (m *customStocktakeModel) FindOne(ctx context.Context, tenantId, stocktakeId int64) (*Stocktake, error) {
	var resp Stocktake
	query := fmt.Sprintf("select %s from %s where `stocktake_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		stocktakeRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, stocktakeId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customStocktakeModel) FindPage(ctx context.Context, tenantId, warehouseId int64, status int64, page, size int64) ([]*Stocktake, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if warehouseId > 0 {
		where += " and `warehouse_id` = ?"
		args = append(args, warehouseId)
	}
	if status > 0 {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `stocktake_id` desc limit ? offset ?",
		stocktakeRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Stocktake
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customStocktakeModel) UpdateStatus(ctx context.Context, tenantId, stocktakeId, status int64, remark string, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `status` = ?, `remark` = case when ? = '' then `remark` else ? end, `updated_by` = ?, `updated_at` = ? where `stocktake_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, status, remark, remark, toNullInt64(updatedBy), time.Now(), stocktakeId, tenantId)
	return err
}
