// price 表 custom 覆写（ADR-08）：版本化价格——SetPrice 只插入新版本行（version+1），历史不删改。
// 生效版本 = 同 (sku_id, price_type, tier_qty) 键内 version 最大。

package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PriceModel = (*customPriceModel)(nil)

type (
	// PriceModel 价格模型（版本化）。
	PriceModel interface {
		// InsertLatest 落库新版本（version = 当前键内最大+1）。
		InsertLatest(ctx context.Context, data *Price) (int64, error)
		// ListBySku SKU 价格（latestOnly=true 时仅各键最新版本）。
		ListBySku(ctx context.Context, tenantId, skuId int64, latestOnly bool) ([]*Price, error)
	}

	customPriceModel struct {
		*defaultPriceModel
	}
)

// NewPriceModel returns a model for the database table.
func NewPriceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PriceModel {
	return &customPriceModel{
		defaultPriceModel: newPriceModel(conn, c, opts...),
	}
}

// InsertLatest 事务内锁键取最大版本 → 插入新行（并发 SetPrice 同键不撞版本号）。
func (m *customPriceModel) InsertLatest(ctx context.Context, data *Price) (int64, error) {
	var version int64
	err := m.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// for update 串行化同键版本分配（键无唯一约束，行锁兜底并发写）
		q := fmt.Sprintf("select coalesce(max(`version`), 0) from %s where `sku_id` = ? and `price_type` = ? and `tier_qty` = ? and `tenant_id` = ? for update", m.table)
		if err := session.QueryRowCtx(ctx, &version, q, data.SkuId, data.PriceType, data.TierQty, data.TenantId); err != nil && err != sqlx.ErrNotFound {
			return err
		}
		version++
		data.Version = version
		ins := fmt.Sprintf("insert into %s (`price_id`, `sku_id`, `price_type`, `tier_qty`, `amount`, `version`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table)
		_, err := session.ExecCtx(ctx, ins, data.PriceId, data.SkuId, data.PriceType, data.TierQty, data.Amount, version, data.TenantId, data.CreatedBy, data.UpdatedBy)
		return err
	})
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (m *customPriceModel) ListBySku(ctx context.Context, tenantId, skuId int64, latestOnly bool) ([]*Price, error) {
	base := "select %s from %s where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	order := " order by `price_type` asc, `tier_qty` asc, `version` desc"
	if latestOnly {
		// 各键最新版本：version = 键内最大（价目表视图）
		q := fmt.Sprintf("select %s from %s p where `sku_id` = ? and `tenant_id` = ? and `deleted_at` is null and `version` = (select max(`version`) from %s p2 where p2.`sku_id` = p.`sku_id` and p2.`price_type` = p.`price_type` and p2.`tier_qty` = p.`tier_qty` and p2.`tenant_id` = p.`tenant_id` and p2.`deleted_at` is null)"+order, priceRows, m.table, m.table)
		var list []*Price
		err := m.QueryRowsNoCacheCtx(ctx, &list, q, skuId, tenantId)
		if err != nil && err != ErrNotFound {
			return nil, err
		}
		return list, nil
	}
	q := fmt.Sprintf(base+order, priceRows, m.table)
	var list []*Price
	err := m.QueryRowsNoCacheCtx(ctx, &list, q, skuId, tenantId)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	return list, nil
}
