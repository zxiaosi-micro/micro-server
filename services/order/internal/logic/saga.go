// Saga 编排内核（02 §9.1，ADR-03/09）：单编排主干，cron 扫描推进，全链无 MQ 延迟消息。
//
// 状态机：CREATED → INVENTORY_LOCKED → PAID → STOCK_OUT → CONTRACT → [STATION] → DONE
//
//	步骤1 创建订单：CreateOrder 本地事务（order+saga 同事务，失败本地回滚）
//	步骤2 锁定库存：inventory.Reserve（幂等键 ORDER_LOCK:orderNo:skuId；补偿=Release）
//	步骤3 发起支付：PayOrder 用户触达 finance.CreatePayment（等待 order_paid 事件推进）
//	步骤4 出库扣减：inventory.DeductLocked（ORDER_OUT；出库成功后不可自动补偿 → 人工队列）
//	步骤5 合同质保：contract.CreateFromOrder（重试至成功）
//	步骤6 建站绑设备：station（S6 落地前恒为可重试失败 → 超限人工队列；仅场站单，非场站单跳过）
//
// 推进语义（ADR-09 三层）：
//   - 加速器：事件消费/RPC 完成后的同步推进（context.WithoutCancel，E5）；
//   - 兜底：cron 扫描 next_retry_at（重试退避 1m/5m/30m，超限 → MANUAL 人工队列）；
//   - 幂等：每步 RPC 幂等（下游 1062=已处理 视为成功）+ 状态推进 CAS（affected=0 幂等跳过）。
//
// 并发纪律：任何一次推进只在事务内持 saga 行锁做状态转移；RPC 动作一律在事务外（不占 DB 连接等下游）。
// 补偿纪律：补偿是新的业务动作（Release），幂等键 ORDER_UNLOCK:orderNo:skuId；
// 出库（步骤4）成功后只能走退货/人工，不做有损自动补偿（02 §9.1 补偿表）。

package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"

	ctpb "micro-server/services/contract/pb"
	stpb "micro-server/services/station/pb"
	finpb "micro-server/services/finance/pb"
	invpb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// Saga 步骤常量（02 §9.1 补偿表）。
const (
	stepCreate   = 1
	stepLock     = 2
	stepPay      = 3
	stepOut      = 4
	stepContract = 5
	stepStation  = 6
)

// stepName / stepIdemKey 供 Saga 进度可视化与审计。
var stepName = map[int64]string{
	stepCreate: "创建订单", stepLock: "锁定库存", stepPay: "发起支付",
	stepOut: "出库扣减", stepContract: "合同质保", stepStation: "建站绑设备",
}

var stepIdemKey = map[int64]string{
	stepCreate: "order_no", stepLock: "saga_id+lock", stepPay: "saga_id+pay",
	stepOut: "saga_id+out", stepContract: "saga_id+contract", stepStation: "saga_id+station",
}

// SagaContext saga.context_json：补偿所需全部原始业务标识（补偿可能晚很久执行，02 §9.1）。
type SagaContext struct {
	LockKeys   []string `json:"lock_keys,omitempty"`   // 已 Reserve 成功的 ORDER_LOCK 幂等键（补偿释放依据）
	PaymentNo  string   `json:"payment_no,omitempty"`  // 发起支付后的支付单号
	PayChannel string   `json:"pay_channel,omitempty"`
	ContractID int64    `json:"contract_id,omitempty"` // 合同号（重试幂等比对）
}

// 库存幂等键约定：inventory stock_record (biz_type, biz_no) 唯一约束兜底；每库存行一键（多 SKU 单不能共用 order_no）。
const (
	bizTypeLock   = "ORDER_LOCK"
	bizTypeUnlock = "ORDER_UNLOCK"
	bizTypeOut    = "ORDER_OUT"
)

func lockBizNo(orderNo string, skuId int64) string { return fmt.Sprintf("%s:%d", orderNo, skuId) }

// rpcTimeout 单次下游 RPC 超时预算（02 §10：内部 RPC 3~5s，禁重试——Saga/事件兜底）。
const rpcTimeout = 5 * time.Second

// stepFailure 步骤失败分类。
type stepFailure struct {
	kind   stepFailKind
	reason string
}

type stepFailKind int

const (
	failRetryable stepFailKind = iota // 退避重试（1m/5m/30m，超限 MANUAL）
	failManual                        // 直接人工队列（出库环节不可自动补偿）
	failCancel                        // 快速失败：补偿已执行部分后取消（库存不足）
)

// rpcCall 带超时预算的下游调用。
func rpcCall(ctx context.Context, fn func(ctx context.Context) error) error {
	rctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	return fn(rctx)
}

// classifyStepErr 下游错误分类：重放=幂等成功，库存不足=快速失败，锁定余量异常=人工，其余=可重试。
func classifyStepErr(err error) (replay bool, failure *stepFailure) {
	if err == nil {
		return false, nil
	}
	code, ok := bizCodeOf(err)
	if ok {
		switch code {
		case codeInvTxnReplay:
			return true, nil // 下游已处理过（幂等成功）
		case codeInvInsufficient:
			return false, &stepFailure{kind: failCancel, reason: "库存不足"}
		case codeInvLockedShort:
			return false, &stepFailure{kind: failManual, reason: "锁定余量不足(数据不一致)"}
		}
	}
	return false, &stepFailure{kind: failRetryable, reason: truncateErr(err)}
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// parseSagaContext saga.context_json 反序列化（空/损坏返回零值——补偿上下文只增不破坏）。
func parseSagaContext(raw sql.NullString) *SagaContext {
	sc := &SagaContext{}
	if raw.Valid && raw.String != "" {
		_ = json.Unmarshal([]byte(raw.String), sc)
	}
	return sc
}

// loadSagaForAdvance 推进入口快照（行锁 + 校验可推进）。返回 nil 表示无需推进（终态/不存在）。
func loadSagaForAdvance(ctx context.Context, sc *svc.ServiceContext, tid int64, orderNo string) (*model.Saga, error) {
	var saga *model.Saga
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		s, err := sc.Models.Saga.FindOneForUpdateTx(ctx, session, tid, orderNo)
		if err != nil {
			return err
		}
		saga = s
		return nil
	})
	if err != nil {
		if err == model.ErrNotFound {
			return nil, nil // 无 Saga（RETURN 等非编排单）：幂等无操作
		}
		return nil, err
	}
	if saga.Status != "RUNNING" {
		return nil, nil // DONE/CANCELLED/MANUAL/COMPENSATING 不推进
	}
	return saga, nil
}

// tryAdvance 推进引擎：从 current_step 连续推进至终态或等待点。
// 自带失败登记：退避 1m/5m/30m（超限 MANUAL）。
func tryAdvance(ctx context.Context, sc *svc.ServiceContext, tid int64, orderNo string) error {
	ctx = context.WithoutCancel(ctx) // E5：事件消费/扫描推进不挂请求 ctx
	log := logx.WithContext(ctx).WithFields(logx.Field("order_no", orderNo))

	for i := 0; i < 8; i++ { // 单次最多推进 6 步，防异常死循环
		saga, err := loadSagaForAdvance(ctx, sc, tid, orderNo)
		if err != nil {
			return err
		}
		if saga == nil {
			return nil
		}

		var failure *stepFailure
		switch saga.CurrentStep {
		case stepCreate:
			// 步骤1 已随下单事务完成 → 推进到步骤2
			failure = casStep(ctx, sc, tid, saga, stepCreate, stepLock, "", nil, nil)
		case stepLock:
			failure = doLockStep(ctx, sc, tid, saga)
		case stepPay:
			// 等待用户支付（PayOrder 触达）与 order_paid 事件；无自动动作。
			// 清退避：next_retry_at=NULL，重试扫描不再空转；超时由 pay_expire_at 扫描负责。
			_ = clearRetryBook(ctx, sc, tid, saga.SagaId)
			log.Infof("saga 停靠步骤3(等待支付)")
			return nil
		case stepOut:
			failure = doOutStep(ctx, sc, tid, saga)
		case stepContract:
			failure = doContractStep(ctx, sc, tid, saga)
		case stepStation:
			failure = doStationStep(ctx, sc, tid, saga)
		default:
			return nil
		}

		if failure == nil {
			continue // 步骤推进成功，继续下一步
		}
		return bookFailure(ctx, sc, tid, saga, failure)
	}
	log.Errorf("saga 推进循环异常退出(超过单次步数上限)")
	return nil
}

// casStep 步骤推进 CAS + 订单状态联动（同事务）。
// orderTo 为空则不动订单状态；extra 在 CAS 成功后于同一事务执行（补偿上下文/事件落 outbox）。
func casStep(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga,
	from, to int64, orderTo string, orderFroms []string, extra func(ctx context.Context, session sqlx.Session) error) *stepFailure {
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Saga.UpdateStepTx(ctx, session, tid, saga.SagaId, from, to); err != nil {
			return err
		}
		if orderTo != "" && len(orderFroms) > 0 {
			if err := sc.Models.Order.CASStatusTx(ctx, session, tid, saga.OrderId, orderFroms, orderTo); err != nil {
				return err
			}
		}
		if err := sc.Models.Saga.ClearRetryTx(ctx, session, tid, saga.SagaId); err != nil {
			return err
		}
		if extra != nil {
			return extra(ctx, session)
		}
		return nil
	})
	if err != nil {
		if err == model.ErrStatusConflict {
			return nil // 并发已推进：幂等跳过
		}
		return &stepFailure{kind: failRetryable, reason: "状态推进失败: " + truncateErr(err)}
	}
	return nil
}

// doLockStep 步骤2：逐 SKU 锁定库存（Reserve 幂等；不足=快速失败+补偿已锁部分）。
func doLockStep(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga) *stepFailure {
	if sc.Inventory == nil {
		return &stepFailure{kind: failRetryable, reason: "inventory RPC 未配置"}
	}
	items, err := sc.Models.OrderItem.FindByOrder(ctx, tid, saga.OrderId)
	if err != nil {
		return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
	}

	var locked []int64 // 本次确认（成功或下游重放）的 SKU（补偿释放依据）
	for _, it := range items {
		err := rpcCall(ctx, func(ctx context.Context) error {
			_, e := sc.Inventory.Reserve(ctx, &invpb.ReserveReq{
				WarehouseId: it.WarehouseId, SkuId: it.SkuId, Qty: int32(it.Qty),
				BizType: bizTypeLock, BizNo: lockBizNo(saga.OrderNo, it.SkuId),
			})
			return e
		})
		_, failure := classifyStepErr(err)
		if failure != nil {
			if failure.kind == failCancel && len(locked) > 0 {
				// 补偿已锁部分（正向步骤内的部分失败；Release 幂等，失败留痕走对账兜底）
				releaseLockedItems(ctx, sc, tid, saga.OrderNo, items, locked, failure.reason)
			}
			return failure
		}
		locked = append(locked, it.SkuId)
	}

	// 步骤2 → 3：订单 LOCKED；持久化锁定键（补偿依据）
	return casStep(ctx, sc, tid, saga, stepLock, stepPay, "LOCKED", []string{"CREATED"},
		func(ctx context.Context, session sqlx.Session) error {
			scx := parseSagaContext(saga.ContextJson)
			for _, sku := range locked {
				scx.LockKeys = append(scx.LockKeys, lockBizNo(saga.OrderNo, sku))
			}
			return sc.Models.Saga.UpdateContextTx(ctx, session, tid, saga.SagaId, mustJSON(scx))
		})
}

// releaseLockedItems 补偿动作：按订单明细释放已锁定库存（幂等；失败仅留痕——对账兜底，E10）。
// onlySku 为 nil 时释放全部明细（取消路径）；非空时仅释放已确认锁定项（部分失败补偿）。
func releaseLockedItems(ctx context.Context, sc *svc.ServiceContext, tid int64, orderNo string, items []*model.OrderItem, onlySku []int64, reason string) {
	if sc.Inventory == nil {
		return
	}
	release := map[int64]bool{}
	for _, sku := range onlySku {
		release[sku] = true
	}
	for _, it := range items {
		if onlySku != nil && !release[it.SkuId] {
			continue
		}
		err := rpcCall(ctx, func(ctx context.Context) error {
			_, e := sc.Inventory.Release(ctx, &invpb.ReleaseReq{
				WarehouseId: it.WarehouseId, SkuId: it.SkuId, Qty: int32(it.Qty),
				BizType: bizTypeUnlock, BizNo: lockBizNo(orderNo, it.SkuId),
			})
			return e
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("saga 补偿释放失败(留待对账) sku=%d: %v", it.SkuId, err)
		}
	}
	logx.WithContext(ctx).Errorf("saga 补偿释放库存完成 order_no=%s reason=%s", orderNo, reason)
}

// doOutStep 步骤4：逐 SKU 出库扣减（DeductLocked；成功后不可自动补偿）。
func doOutStep(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga) *stepFailure {
	if sc.Inventory == nil {
		return &stepFailure{kind: failRetryable, reason: "inventory RPC 未配置"}
	}
	items, err := sc.Models.OrderItem.FindByOrder(ctx, tid, saga.OrderId)
	if err != nil {
		return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
	}

	for _, it := range items {
		if it.OutQty >= it.Qty {
			continue // 已确认（RPC/事件双路回写，幂等）
		}
		pending := it.Qty - it.OutQty
		// SN 明细随出库事件下发（S6-01：device 消费 stock_out 驱动状态机 OUT；DEVICE 单每行一 SN）
		var sns []string
		if it.Sn.Valid && it.Sn.String != "" {
			sns = []string{it.Sn.String}
		}
		err := rpcCall(ctx, func(ctx context.Context) error {
			_, e := sc.Inventory.DeductLocked(ctx, &invpb.DeductLockedReq{
				WarehouseId: it.WarehouseId, SkuId: it.SkuId, Qty: int32(pending),
				BizType: bizTypeOut, BizNo: lockBizNo(saga.OrderNo, it.SkuId), Sns: sns,
			})
			return e
		})
		_, failure := classifyStepErr(err)
		if failure != nil {
			// 出库环节业务失败（余量不足等）：不可自动补偿 → 人工介入队列（02 §9.1 补偿表）
			return failure
		}
		// 出库确认回写（per (order,sku) 幂等：out_qty 累计不越过 qty）
		err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			_, err := sc.Models.OrderItem.IncOutQtyTx(ctx, session, tid, saga.OrderId, it.SkuId, pending)
			return err
		})
		if err != nil {
			return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
		}
	}

	// 全部出库确认 → 步骤5：订单 STOCK_OUT
	return casStep(ctx, sc, tid, saga, stepOut, stepContract, "STOCK_OUT", []string{"PAID"}, nil)
}

// doContractStep 步骤5：合同+质保建立（CreateFromOrder 幂等：同单重放返回已有合同；重试至成功）。
func doContractStep(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga) *stepFailure {
	if sc.Contract == nil {
		return &stepFailure{kind: failRetryable, reason: "contract RPC 未配置"}
	}
	order, err := sc.Models.Order.FindOneScoped(ctx, tid, saga.OrderId)
	if err != nil {
		return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
	}
	items, err := sc.Models.OrderItem.FindByOrder(ctx, tid, saga.OrderId)
	if err != nil {
		return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
	}

	type itemVO struct {
		SkuId int64  `json:"sku_id"`
		Sn    string `json:"sn,omitempty"`
		Qty   int32  `json:"qty"`
	}
	ivs := make([]itemVO, 0, len(items))
	for _, it := range items {
		ivs = append(ivs, itemVO{SkuId: it.SkuId, Sn: nullStr(it.Sn), Qty: int32(it.Qty)})
	}

	contractType := "SALES"
	if saga.OrderType == "STATION" {
		contractType = "STATION"
	}
	var res *ctpb.CreateFromOrderResp
	err = rpcCall(ctx, func(ctx context.Context) error {
		var e error
		res, e = sc.Contract.CreateFromOrder(ctx, &ctpb.CreateFromOrderReq{
			OrderNo: saga.OrderNo, OrderType: contractType,
			BuyerPartyId: order.BuyerPartyId.Int64, Amount: centsToAmount(floatToCents(order.TotalAmount)),
			ItemsJson: mustJSON(ivs),
		})
		return e
	})
	_, failure := classifyStepErr(err)
	if failure != nil {
		// 合同域无"不足"类失败：一律重试至成功（02 §9.1 步骤5 补偿=重试）
		if failure.kind == failCancel {
			failure.kind = failRetryable
		}
		return failure
	}
	contractID := int64(0)
	if res != nil {
		contractID = res.ContractId
	}

	// 步骤5 → 6：订单 CONTRACTED（场站单继续步骤6；非场站单直接终态）
	return casStep(ctx, sc, tid, saga, stepContract, stepStation, "CONTRACTED", []string{"STOCK_OUT"},
		func(ctx context.Context, session sqlx.Session) error {
			scx := parseSagaContext(saga.ContextJson)
			scx.ContractID = contractID
			if err := sc.Models.Saga.UpdateContextTx(ctx, session, tid, saga.SagaId, mustJSON(scx)); err != nil {
				return err
			}
			if saga.OrderType != "STATION" {
				// 非场站单：步骤6 跳过 → 终态
				if err := sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "DONE", ""); err != nil {
					return err
				}
				if err := sc.Models.Order.CASStatusTx(ctx, session, tid, saga.OrderId,
					[]string{"CONTRACTED"}, "DONE"); err != nil && err != model.ErrStatusConflict {
					return err
				}
			}
			return nil
		})
}

// doStationStep 步骤6：建站+绑设备（场站单；station.CreateStation 幂等键=order_no，
// 重放直接返回已有场站；补偿口径 02 §9.1：重试 + 人工队列）。
func doStationStep(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga) *stepFailure {
	if sc.Station == nil {
		return &stepFailure{kind: failRetryable, reason: "station RPC 未配置"}
	}
	items, err := sc.Models.OrderItem.FindByOrder(ctx, tid, saga.OrderId)
	if err != nil {
		return &stepFailure{kind: failRetryable, reason: truncateErr(err)}
	}
	binds := make([]*stpb.StationDeviceBind, 0, len(items))
	for _, it := range items {
		if it.Sn.Valid && it.Sn.String != "" {
			binds = append(binds, &stpb.StationDeviceBind{Sn: it.Sn.String, Role: "PACK"})
		}
	}
	var res *stpb.CreateStationResp
	err = rpcCall(ctx, func(ctx context.Context) error {
		var e error
		res, e = sc.Station.CreateStation(ctx, &stpb.CreateStationReq{
			OrderNo: saga.OrderNo,
			Name:    "场站-" + saga.OrderNo,
			Type:    "ESS",
			Devices: binds,
		})
		return e
	})
	_, failure := classifyStepErr(err)
	if failure != nil {
		if failure.kind == failCancel {
			failure.kind = failRetryable // 建站无"不足"类失败：一律重试至成功
		}
		return failure
	}
	logx.WithContext(ctx).Infof("saga 步骤6 建站完成 order_no=%s station_id=%d devices=%d",
		saga.OrderNo, res.StationId, len(binds))
	return nil
}

// bookFailure 失败登记：退避重试 / 人工队列 / 快速失败补偿取消。
func bookFailure(ctx context.Context, sc *svc.ServiceContext, tid int64, saga *model.Saga, failure *stepFailure) error {
	log := logx.WithContext(ctx).WithFields(logx.Field("order_no", saga.OrderNo), logx.Field("step", saga.CurrentStep))
	return sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		switch failure.kind {
		case failCancel:
			// 快速失败（库存不足）：取消（锁定键只在成功后持久化，此处无可释放库存）
			if err := sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "CANCELLED", failure.reason); err != nil {
				return err
			}
			if err := sc.Models.Order.CASStatusTx(ctx, session, tid, saga.OrderId,
				[]string{"CREATED", "LOCKED", "PAYING"}, "CANCELLED"); err != nil && err != model.ErrStatusConflict {
				return err
			}
			return eventbus.Emit(ctx, session, orderCancelledEvent(tid, saga.OrderNo, failure.reason, "SAGA"))
		case failManual:
			if err := sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "MANUAL", failure.reason); err != nil {
				return err
			}
			log.Errorf("saga 进入人工队列 step=%d reason=%s", saga.CurrentStep, failure.reason)
			return nil
		default:
			retry := saga.RetryCount + 1
			if int(retry) > maxRetryOf(sc) {
				if err := sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "MANUAL", failure.reason); err != nil {
					return err
				}
				log.Errorf("saga 重试超限进入人工队列 step=%d retries=%d reason=%s", saga.CurrentStep, retry, failure.reason)
				return nil
			}
			return sc.Models.Saga.MarkRetryTx(ctx, session, tid, saga.SagaId, retry,
				time.Now().Add(backoffOf(sc, int(retry))), failure.reason)
		}
	})
}

func clearRetryBook(ctx context.Context, sc *svc.ServiceContext, tid, sagaId int64) error {
	return sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return sc.Models.Saga.ClearRetryTx(ctx, session, tid, sagaId)
	})
}

// ---- 事件消费后的推进入口（bridge 调用；下一状态已落库，由 tryAdvance 连续推进）----

// KickAdvance 事件/RPC 落库后的推进触发（异步，WithoutCancel——失败由 cron 重试扫描兜底）。
func KickAdvance(sc *svc.ServiceContext, tid int64, orderNo string) {
	ctx := context.WithoutCancel(context.Background())
	ctx = tenantxWith(ctx, tid)
	go func() {
		if err := tryAdvance(ctx, sc, tid, orderNo); err != nil {
			logx.WithContext(ctx).Errorf("saga 异步推进失败 order_no=%s: %v", orderNo, err)
		}
	}()
}

// ---- cron 推进器 ②（注册见 cron.go）：重试扫描 ----

// ScanSagaRetry next_retry_at 到期的 RUNNING saga 重新推进（兜底路径）。
// 扫描 SQL 无租户条件（跨租户）；推进按 saga 归属租户重建 ctx——下游 RPC 需要租户元数据（E5 不挂请求 ctx）。
func ScanSagaRetry(ctx context.Context, sc *svc.ServiceContext) error {
	due, err := sc.Models.Saga.FindRetryDue(ctx, time.Now(), advanceBatchOf(sc))
	if err != nil {
		return err
	}
	for _, saga := range due {
		sctx := tenantxWith(context.WithoutCancel(context.Background()), saga.TenantId)
		if err := tryAdvance(sctx, sc, saga.TenantId, saga.OrderNo); err != nil {
			logx.WithContext(ctx).Errorf("saga 重试推进失败 order_no=%s: %v", saga.OrderNo, err)
		}
	}
	return nil
}

// ---- 配置取值 ----

func maxRetryOf(sc *svc.ServiceContext) int {
	if sc.Config.SagaMaxRetry > 0 {
		return sc.Config.SagaMaxRetry
	}
	return 3
}

func backoffOf(sc *svc.ServiceContext, retry int) time.Duration {
	bo := sc.Config.SagaRetryBackoffMinutes
	if len(bo) == 0 {
		bo = []int{1, 5, 30}
	}
	idx := retry - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(bo) {
		idx = len(bo) - 1
	}
	return time.Duration(bo[idx]) * time.Minute
}

var _ = finpb.CreatePaymentReq{} // finance 契约引用（PayOrder 在 pay_order_logic.go）
