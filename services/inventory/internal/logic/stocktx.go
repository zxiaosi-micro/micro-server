// 防超卖双保险核心（02 §9.2）：
//
//	Reserve 流程：
//	  1. Redis DECRBY inv:{tid}:{wid}:{sku} 原子预扣（负值→INCRBY 回滚+返回不足，挡并发）
//	  2. DB 事务：行锁取 before 快照 → 余量守卫 UPDATE（§6.2 样例，affected==0 即不足）
//	     → stock_record 流水 + Outbox 事件（同事务）
//	  3. 幂等键 (biz_type,biz_no) 唯一约束，1062 → ErrTxnReplay
//	  4. Redis key 缺失/故障 → 跳过 1 纯 DB 路径（同样不超卖）
//
// 本文件同时提供各库存变动共用的事务编排（行锁→变更→流水→outbox→事件）与 Redis 同步策略。

package logic

import (
	"context"
	"strconv"

	"micro-server/services/inventory/internal/confcenter"
	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// gateState 预扣网关三态。
type gateState int

const (
	gateDegrade gateState = iota // 网关未就绪 → 降级纯 DB（02 §9.2 第 4 步）
	gatePass                     // 预扣成功 → 走 DB；失败须 rollbackGate 归还
	gateReject                   // 计数不足（负值已回滚）→ 直接拒绝，不打 DB
)

// stockGate Redis 预扣网关（双保险第 1 步）。
// 拒绝必须显式短路：负值拒绝若落入 DB 路径，业务错误会被 go-zero 熔断器计为失败率，
// 并发拒绝风暴会误开熔断——这也是"预扣挡并发"的意义所在。
func stockGate(ctx context.Context, sc *svc.ServiceContext, tid, wid, skuId int64, qty int64) gateState {
	if sc.Rd == nil || !confcenter.Current().RedisGateEnabled {
		return gateDegrade
	}
	key := redisKey(tid, wid, skuId)
	// key 缺失视为网关未就绪 → 降级纯 DB（缺失 key 的 DECRBY 会从 0 起算，误判不足）
	ok, err := sc.Rd.ExistsCtx(ctx, key)
	if err != nil {
		logx.WithContext(ctx).Errorf("stockGate: Redis EXISTS 降级 key=%s: %v", key, err)
		return gateDegrade
	}
	if !ok {
		return gateDegrade
	}
	n, err := sc.Rd.DecrbyCtx(ctx, key, qty)
	if err != nil {
		// 连接故障 → 降级纯 DB（DB 余量守卫同样不超卖，02 §9.2 第 4 步）
		logx.WithContext(ctx).Errorf("stockGate: Redis 预扣降级 key=%s: %v", key, err)
		return gateDegrade
	}
	if n < 0 {
		// 负值 → INCRBY 回滚 + 显式拒绝（挡并发主路径）
		if _, ierr := sc.Rd.IncrbyCtx(ctx, key, qty); ierr != nil {
			logx.WithContext(ctx).Errorf("stockGate: Redis 负值回滚失败 key=%s: %v（等待对账收敛）", key, ierr)
		}
		return gateReject
	}
	return gatePass
}

// rollbackGate 归还预扣（DB 事务失败时调用；失败仅记日志，对账 cron 兜底收敛）。
func rollbackGate(ctx context.Context, sc *svc.ServiceContext, tid, wid, skuId int64, qty int64) {
	if sc.Rd == nil || !confcenter.Current().RedisGateEnabled {
		return
	}
	key := redisKey(tid, wid, skuId)
	if _, err := sc.Rd.IncrbyCtx(ctx, key, qty); err != nil {
		logx.WithContext(ctx).Errorf("rollbackGate: Redis 归还失败 key=%s: %v（等待对账收敛）", key, err)
	}
}

// syncGateInc 入库/释放类提交后按增量同步计数器（available 增加方向，best-effort）。
func syncGateInc(ctx context.Context, sc *svc.ServiceContext, tid, wid, skuId int64, qty int64) {
	if sc.Rd == nil || qty <= 0 {
		return
	}
	key := redisKey(tid, wid, skuId)
	if _, err := sc.Rd.IncrbyCtx(ctx, key, qty); err != nil {
		logx.WithContext(ctx).Errorf("syncGateInc: Redis 同步失败 key=%s: %v（等待对账收敛）", key, err)
	}
}

// syncGateSet 低频可用性调整类（备件/盘点）提交后以 DB 为准校正计数器（best-effort）。
func syncGateSet(ctx context.Context, sc *svc.ServiceContext, tid, wid, skuId int64, available int64) {
	if sc.Rd == nil {
		return
	}
	key := redisKey(tid, wid, skuId)
	if err := sc.Rd.SetCtx(ctx, key, strconv.FormatInt(available, 10)); err != nil {
		logx.WithContext(ctx).Errorf("syncGateSet: Redis 校正失败 key=%s: %v（等待对账收敛）", key, err)
	}
}

// redisKey 预扣计数器键（02 §9.2 inv:{wid}:{sku} 扩展租户前缀隔离计数空间）。
func redisKey(tid, wid, skuId int64) string {
	return "inv:" + itoa(tid) + ":" + itoa(wid) + ":" + itoa(skuId)
}

// applyStockTx 共用事务编排：行锁取 before 快照 → mutate（守卫式变更，返回本次操作量）
// → 流水（幂等键 1062 → ErrTxnReplay）→ outbox 事件（同事务）。
// eventFn 基于 before/after 状态构造事件（可为 nil）；事件与业务变更同事务提交。
func applyStockTx(ctx context.Context, sc *svc.ServiceContext, tid, op int64,
	wid, skuId int64, bizType, bizNo, remark string,
	mutate func(ctx context.Context, session sqlx.Session, inv *model.Inventory) (int64, error),
	eventFn func(before, after *model.Inventory) []eventbus.EmitInput) (int64, error) {

	var recordId int64
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// 行锁 + before 快照（未建档明确报错）
		inv, err := sc.Models.Inventory.LockByWhSku(ctx, session, tid, wid, skuId)
		if err != nil {
			if err == model.ErrNotFound {
				return errInventoryNotFound
			}
			return err
		}
		before := *inv

		// 守卫式变更（affected==0 → 余量不足）；qty 为本次操作量（操作语义口径）
		qty, err := mutate(ctx, session, inv)
		if err != nil {
			return err
		}

		// 流水（同事务；幂等键 1062 → ErrTxnReplay）
		recordId = sc.Snowflake.MustNextID()
		rec := &model.StockRecord{
			RecordId:        recordId,
			InventoryId:     inv.InventoryId,
			WarehouseId:     wid,
			SkuId:           skuId,
			BizType:         bizType,
			BizNo:           bizNo,
			Qty:             qty,
			BeforeAvailable: before.Available,
			AfterAvailable:  inv.Available,
			BeforeLocked:    before.Locked,
			AfterLocked:     inv.Locked,
			Remark:          toNullString(remark),
			TenantId:        tid,
			CreatedBy:       toNullInt64(op),
			UpdatedBy:       toNullInt64(op),
		}
		if err := sc.Models.StockRecord.InsertInTx(ctx, session, rec); err != nil {
			if isDupKey(err) {
				return ErrTxnReplay
			}
			return err
		}

		// Outbox 事件（与业务变更同事务；Relay 属 S5 落地）
		if eventFn != nil {
			for _, ev := range eventFn(&before, inv) {
				if err := eventbus.Emit(ctx, session, ev); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return recordId, nil
}

// stockLowEvents 低库存事件构造（edge 触发：before 达标且 after 低于阈值才发，避免刷屏）。
func stockLowEvents(tid, wid, skuId int64, before, after *model.Inventory) []eventbus.EmitInput {
	if after.LowStockThreshold <= 0 {
		return nil
	}
	if before.Available >= after.LowStockThreshold && after.Available < after.LowStockThreshold {
		return []eventbus.EmitInput{{
			Topic:    eventbus.TopicStockLow,
			Type:     eventbus.TypeStockLow,
			Key:      itoa(wid) + ":" + itoa(skuId),
			TenantID: tid,
			Payload: map[string]any{
				"tenant_id": tid, "warehouse_id": wid, "sku_id": skuId,
				"available": after.Available, "threshold": after.LowStockThreshold,
			},
		}}
	}
	return nil
}

// stockOutEvent 出库事件（DeductLocked 提交时发出，01 §7.3；S6 起 sns 供 device 状态机驱动 OUT）。
func stockOutEvent(tid, wid, skuId int64, qty int64, bizNo string, sns []string) eventbus.EmitInput {
	if sns == nil {
		sns = []string{}
	}
	return eventbus.EmitInput{
		Topic:    eventbus.TopicStockOut,
		Type:     eventbus.TypeStockOut,
		Key:      itoa(wid) + ":" + itoa(skuId),
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "warehouse_id": wid, "sku_id": skuId,
			"qty": qty, "biz_no": bizNo, "sns": sns,
		},
	}
}

// stockInEvent 入库事件（StockIn 提交时发出，S6-01：device 消费驱动状态机 IN_STOCK）。
func stockInEvent(tid, wid, skuId int64, qty int64, bizNo string, sns []string) eventbus.EmitInput {
	if sns == nil {
		sns = []string{}
	}
	return eventbus.EmitInput{
		Topic:    eventbus.TopicStockIn,
		Type:     eventbus.TypeStockIn,
		Key:      itoa(wid) + ":" + itoa(skuId),
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "warehouse_id": wid, "sku_id": skuId,
			"qty": qty, "biz_no": bizNo, "sns": sns,
		},
	}
}

// mapTxErr 统一事务错误出口（业务码直传，其余兜底 Internal 不泄细节）。
func mapTxErr(err error) error {
	if err == nil {
		return nil
	}
	switch err {
	case ErrInsufficientStock, ErrLockedInsufficient, ErrTxnReplay, errInventoryNotFound:
		return err
	}
	return errcode.Internal.WithCause(err)
}
