// dealer_ext 表 custom 覆写（ADR-08）：1:1 扩展表，PK 即 party_id；Upsert 幂等。
// 注意：ON DUPLICATE KEY UPDATE 走主键冲突路径，需显式含 tenant_id 列（tenantaudit 口径）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DealerExtModel = (*customDealerExtModel)(nil)

type (
	// DealerExtModel 经销商扩展模型。
	DealerExtModel interface {
		// Upsert 按 PK(party_id) 幂等写入/更新。
		Upsert(ctx context.Context, data *DealerExt, updatedBy int64) error
		// FindOne 按 party_id 取（租户过滤）；未建档返回 ErrNotFound。
		FindOne(ctx context.Context, tenantId, partyId int64) (*DealerExt, error)
	}

	customDealerExtModel struct {
		*defaultDealerExtModel
	}
)

// NewDealerExtModel returns a model for the database table.
func NewDealerExtModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DealerExtModel {
	return &customDealerExtModel{
		defaultDealerExtModel: newDealerExtModel(conn, c, opts...),
	}
}

func (m *customDealerExtModel) Upsert(ctx context.Context, data *DealerExt, updatedBy int64) error {
	query := fmt.Sprintf(`insert into %s (`+"`party_id`"+`, `+"`dealer_level`"+`, `+"`authorized_region`"+`, `+"`rebate_rule`"+`,
		`+"`tenant_id`"+`, `+"`created_by`"+`, `+"`updated_by`"+`)
		values (?, ?, ?, ?, ?, ?, ?)
		on duplicate key update `+"`dealer_level`"+` = values(`+"`dealer_level`"+`), `+"`authorized_region`"+` = values(`+"`authorized_region`"+`),
		`+"`rebate_rule`"+` = values(`+"`rebate_rule`"+`), `+"`updated_by`"+` = values(`+"`updated_by`"+`), `+"`updated_at`"+` = ?`, m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, data.PartyId, data.DealerLevel, data.AuthorizedRegion, data.RebateRule,
		data.TenantId,
		sql.NullInt64{Int64: data.CreatedBy.Int64, Valid: data.CreatedBy.Valid},
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0},
		time.Now())
	return err
}

func (m *customDealerExtModel) FindOne(ctx context.Context, tenantId, partyId int64) (*DealerExt, error) {
	var resp DealerExt
	query := fmt.Sprintf("select %s from %s where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		dealerExtRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, partyId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
