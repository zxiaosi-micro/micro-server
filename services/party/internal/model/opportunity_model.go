// opportunity 表 custom 覆写（ADR-08）：商机看板（阶段列）数据源；stage VARCHAR 枚举。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OpportunityModel = (*customOpportunityModel)(nil)

type (
	// OpportunityModel 商机模型。
	OpportunityModel interface {
		// Insert 新建商机（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *Opportunity) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, opportunityId int64) (*Opportunity, error)
		// FindPage 商机列表（partyId 0=全部；stage 空=全部；keyword 模糊 title）。
		FindPage(ctx context.Context, tenantId, partyId int64, stage, keyword string, page, size int64) ([]*Opportunity, int64, error)
		// UpdateStage 阶段推进（看板拖列）。
		UpdateStage(ctx context.Context, tenantId, opportunityId int64, stage string, updatedBy int64) error
	}

	customOpportunityModel struct {
		*defaultOpportunityModel
	}
)

// NewOpportunityModel returns a model for the database table.
func NewOpportunityModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OpportunityModel {
	return &customOpportunityModel{
		defaultOpportunityModel: newOpportunityModel(conn, c, opts...),
	}
}

func (m *customOpportunityModel) FindOne(ctx context.Context, tenantId, opportunityId int64) (*Opportunity, error) {
	var resp Opportunity
	query := fmt.Sprintf("select %s from %s where `opportunity_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		opportunityRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, opportunityId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customOpportunityModel) FindPage(ctx context.Context, tenantId, partyId int64, stage, keyword string, page, size int64) ([]*Opportunity, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if partyId > 0 {
		where += " and `party_id` = ?"
		args = append(args, partyId)
	}
	if stage != "" {
		where += " and `stage` = ?"
		args = append(args, stage)
	}
	if keyword != "" {
		where += " and `title` like ?"
		args = append(args, "%"+keyword+"%")
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `opportunity_id` desc limit ? offset ?",
		opportunityRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Opportunity
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customOpportunityModel) UpdateStage(ctx context.Context, tenantId, opportunityId int64, stage string, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `stage` = ?, `updated_by` = ?, `updated_at` = ? where `opportunity_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, stage,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), opportunityId, tenantId)
	return err
}
