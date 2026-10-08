// warranty_policy 表 custom 覆写（ADR-08）：按 SKU 1:1；Upsert 幂等。
// start_rule=ACTIVATION/RECEIPT，由合同域执行时快照（01 FR-CTL-005）。

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ WarrantyPolicyModel = (*customWarrantyPolicyModel)(nil)

type (
	// WarrantyPolicyModel 质保策略模型。
	WarrantyPolicyModel interface {
		// Upsert 按 sku_id 1:1 幂等写入/更新。
		Upsert(ctx context.Context, data *WarrantyPolicy) error
		// FindOneBySku 按 SKU 取（租户过滤）。
		FindOneBySku(ctx context.Context, tenantId, skuId int64) (*WarrantyPolicy, error)
	}

	customWarrantyPolicyModel struct {
		*defaultWarrantyPolicyModel
	}
)

// NewWarrantyPolicyModel returns a model for the database table.
func NewWarrantyPolicyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) WarrantyPolicyModel {
	return &customWarrantyPolicyModel{
		defaultWarrantyPolicyModel: newWarrantyPolicyModel(conn, c, opts...),
	}
}

func (m *customWarrantyPolicyModel) Upsert(ctx context.Context, data *WarrantyPolicy) error {
	query := "insert into `warranty_policy` (`policy_id`, `sku_id`, `period_months`, `start_rule`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?) on duplicate key update `period_months` = values(`period_months`), `start_rule` = values(`start_rule`), `updated_by` = values(`updated_by`), `updated_at` = ?"
	_, err := m.ExecNoCacheCtx(ctx, query, data.PolicyId, data.SkuId, data.PeriodMonths, data.StartRule,
		data.TenantId, data.CreatedBy, data.UpdatedBy, time.Now())
	return err
}

func (m *customWarrantyPolicyModel) FindOneBySku(ctx context.Context, tenantId, skuId int64) (*WarrantyPolicy, error) {
	var resp WarrantyPolicy
	query := fmt.Sprintf("select %s from %s where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		warrantyPolicyRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, skuId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
