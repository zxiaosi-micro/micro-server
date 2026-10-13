// reconcileRedis Redis 预扣计数器 ↔ DB available 对账（cron inventory-redis-reconcile）。
//
// 背景：预扣回滚 INCRBY 失败等极端路径会造成计数器漂移——Redis 只是"挡并发"的加速层，
// DB 守卫才是权威；本任务周期以 DB 值重置计数器（in-flight 竞态窗口可接受，
// 方向安全：DB 守卫保证不超卖，计数器偏差最多引发误拒/放行至 DB 层被守卫拦下）。

package logic

import (
	"context"

	"micro-server/services/inventory/internal/svc"

	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// ReconcileRedis 预扣计数器自愈：以 DB available 权威值周期重置（漂移窗口自愈）；
// 预扣网关未启用（Rd nil）直接返回。
func ReconcileRedis(ctx context.Context, sc *svc.ServiceContext) error {
	if sc.Rd == nil {
		return nil
	}
	// 逐租户扫描不现实（无租户上下文），按 inventory 全表分批（dev 规模内足够）；
	// 后台作业显式豁免租户过滤（tenantx.Skip，02 §9.4 后台作业口径）。
	ctx = tenantx.Skip(ctx)
	var rows []*inventoryRow
	query := "select `warehouse_id`, `sku_id`, `available`, `tenant_id` from `inventory` where `deleted_at` is null limit 500"
	if err := sc.Conn.QueryRowsCtx(ctx, &rows, query); err != nil {
		return err
	}
	for _, r := range rows {
		// 计数器键含租户前缀；对账按行重置（键 = inv:{tid}:{wid}:{sku}，tid 从行取）
		if err := sc.Rd.SetCtx(ctx, redisKey(r.TenantId, r.WarehouseId, r.SkuId), itoa(r.Available)); err != nil {
			return err
		}
	}
	return nil
}

// inventoryRow 对账扫描行（DB 权威值，用于重置 Redis 计数器）。
type inventoryRow struct {
	WarehouseId int64 `db:"warehouse_id"` // 仓库 ID
	SkuId       int64 `db:"sku_id"`       // SKU ID
	Available   int64 `db:"available"`    // DB 可用量（权威值）
	TenantId    int64 `db:"tenant_id"`    // 租户 ID（计数器键前缀）
}
