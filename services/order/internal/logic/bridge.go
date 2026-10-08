// 事件消费入口（consumer 包经 eventbus.Subscriber 调达；ctx 已恢复租户与 trace）。
// 失败语义：handler 错误 → 去重事务回滚 → event_retry 退避重投（Subscriber 承担）。
// 幂等语义：全部落库走 CAS/守卫更新，重复投递无副作用。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// HandleOrderPaid order_paid（finance 发出）：步骤3 → 4，订单 PAID，触发后续推进。
func HandleOrderPaid(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			OrderNo    string `json:"order_no"`
			PaymentNo  string `json:"payment_no"`
			Channel    string `json:"channel"`
			PaidAtMilli int64 `json:"paid_at"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("order_paid 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		scoped := logx.WithContext(ctx).WithFields(logx.Field("order_no", p.OrderNo), logx.Field("payment_no", p.PaymentNo))

		err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			saga, err := sc.Models.Saga.FindOneForUpdateTx(ctx, session, tid, p.OrderNo)
			if err != nil {
				return err
			}
			// 幂等/竞态：已过步骤3 → 重复或迟到事件，直接 ack
			if saga.CurrentStep > stepPay || saga.Status == "DONE" || saga.Status == "CANCELLED" {
				scoped.Infof("order_paid 迟到/重复（step=%d status=%s）跳过", saga.CurrentStep, saga.Status)
				return nil
			}
			if saga.Status == "COMPENSATING" {
				// 支付成功与取消竞态：入账已完成，需人工/退货处理（对账兜底，E10）
				scoped.Errorf("order_paid 与取消竞态 order_no=%s payment_no=%s —— 需人工核对退款", p.OrderNo, p.PaymentNo)
				return nil
			}
			if saga.CurrentStep < stepPay {
				// 支付先于锁定完成（异常序）：留重投，等锁库步骤追上
				return fmt.Errorf("order_paid 早于库存锁定 step=%d", saga.CurrentStep)
			}
			if err := sc.Models.Saga.UpdateStepTx(ctx, session, tid, saga.SagaId, stepPay, stepOut); err != nil {
				if err == model.ErrStatusConflict {
					return nil // 并发已推进
				}
				return err
			}
			order, err := sc.Models.Order.FindOneByNoForUpdateTx(ctx, session, tid, p.OrderNo)
			if err != nil {
				return err
			}
			if err := sc.Models.Order.CASStatusTx(ctx, session, tid, order.OrderId,
				[]string{"LOCKED", "PAYING"}, "PAID"); err != nil && err != model.ErrStatusConflict {
				return err
			}
			paidAt := timeFromMilli(p.PaidAtMilli)
			if err := sc.Models.Order.MarkPaidTx(ctx, session, tid, order.OrderId, paidAt); err != nil {
				return err
			}
			scx := parseSagaContext(saga.ContextJson)
			if p.PaymentNo != "" {
				scx.PaymentNo = p.PaymentNo
			}
			if p.Channel != "" {
				scx.PayChannel = p.Channel
			}
			if err := sc.Models.Saga.UpdateContextTx(ctx, session, tid, saga.SagaId, mustJSON(scx)); err != nil {
				return err
			}
			// 推进兜底：进程在此刻死亡时 cron 重试扫描接续（ADR-09）
			return sc.Models.Saga.MarkDueTx(ctx, session, tid, saga.SagaId)
		})
		if err != nil {
			return err
		}

		scoped.Infof("order_paid 已确认，Saga 步骤3→4")
		KickAdvance(sc, tid, p.OrderNo)
		return nil
	}
}

// HandleStockOut stock_out（inventory 发出）：出库确认回写 out_qty（per (order,sku) 幂等）。
// biz_no 契约：order 发出的 DeductLocked 用 <order_no>:<sku_id>。
func HandleStockOut(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			OrderNo string `json:"biz_no"`
			SkuId   int64  `json:"sku_id"`
			Qty     int64  `json:"qty"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("stock_out 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		idx := strings.LastIndexByte(p.OrderNo, ':')
		if idx < 0 {
			logx.WithContext(ctx).Errorf("stock_out biz_no 非本域格式 biz_no=%s（跳过）", p.OrderNo)
			return nil
		}
		orderNo := p.OrderNo[:idx]
		// skuId 冗余在 payload；biz_no 尾段为权威
		var bizSkuId int64
		_, _ = fmt.Sscanf(p.OrderNo[idx+1:], "%d", &bizSkuId)
		skuId := p.SkuId
		if bizSkuId > 0 {
			skuId = bizSkuId
		}

		order, err := sc.Models.Order.FindOneByNo(ctx, tid, orderNo)
		if err != nil {
			if err == model.ErrNotFound {
				logx.WithContext(ctx).Errorf("stock_out 订单不存在 order_no=%s（跳过）", orderNo)
				return nil
			}
			return err
		}
		err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			_, err := sc.Models.OrderItem.IncOutQtyTx(ctx, session, tid, order.OrderId, skuId, p.Qty)
			return err
		})
		if err != nil {
			return err
		}
		// 出库确认可能补齐全部明细 → 触发推进（幂等）
		KickAdvance(sc, tid, orderNo)
		return nil
	}
}

// HandlePaymentRefunded payment_refunded（finance 发出）：退货单 REFUNDED 回写（FR-ORD-004）。
func HandlePaymentRefunded(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			OrderNo  string `json:"order_no"`
			ReturnNo string `json:"return_no"`
			RefundNo string `json:"refund_no"`
			Amount   string `json:"amount"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("payment_refunded 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		if p.ReturnNo == "" {
			logx.WithContext(ctx).Infof("payment_refunded 无退货单关联（手工退款）refund_no=%s 跳过", p.RefundNo)
			return nil
		}
		err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.ReturnOrder.MarkRefundedTx(ctx, session, tid, p.ReturnNo, p.RefundNo)
		})
		if err != nil {
			if err == model.ErrStatusConflict {
				logx.WithContext(ctx).Infof("payment_refunded 重复回写 return_no=%s（幂等跳过）", p.ReturnNo)
				return nil
			}
			return err
		}
		logx.WithContext(ctx).Infof("退货退款已回写 return_no=%s refund_no=%s", p.ReturnNo, p.RefundNo)
		return nil
	}
}
