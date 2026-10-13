// finance 事件构造（S5-01 契约）：topic 下划线 / event_type 点号 / key=order_no（保序聚合键）。

package logic

import "github.com/zxiaosi-micro/micro-common/eventbus"

// orderPaidPayload order_paid 事件体（order 消费 → Saga 步骤 3→4 推进）。
type orderPaidPayload struct {
	TenantID   int64  `json:"tenant_id"`   // 租户 ID
	OrderNo    string `json:"order_no"`    // 订单号（事件 key，同单保序）
	PaymentNo  string `json:"payment_no"`  // 支付单号（消费侧幂等键）
	Channel    string `json:"channel"`     // 支付渠道
	PaidAmount string `json:"paid_amount"` // 实付金额（DECIMAL 元字符串）
	PaidAt     int64  `json:"paid_at"`     // 支付完成时刻（UnixMilli）
}

// paymentRefundedPayload payment_refunded 事件体（order 消费 → 退货单 REFUNDED 回写）。
type paymentRefundedPayload struct {
	TenantID  int64  `json:"tenant_id"`  // 租户 ID
	OrderNo   string `json:"order_no"`   // 订单号（事件 key）
	PaymentNo string `json:"payment_no"` // 原支付单号
	ReturnNo  string `json:"return_no"`  // 退货单号（空=手工退款）
	RefundNo  string `json:"refund_no"`  // 退款单号（消费侧幂等键）
	Amount    string `json:"amount"`     // 退款金额（DECIMAL 元字符串）
	Channel   string `json:"channel"`    // 原路退回渠道
}

// orderPaidEvent 组装 order_paid 事件。
func orderPaidEvent(tid int64, orderNo, paymentNo, channel, paidAmount string, paidAtMilli int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderPaid, Type: eventbus.TypeOrderPaid,
		Key: orderNo, TenantID: tid,
		Payload: orderPaidPayload{
			TenantID: tid, OrderNo: orderNo, PaymentNo: paymentNo, Channel: channel,
			PaidAmount: paidAmount, PaidAt: paidAtMilli,
		},
		EventID: eventbus.NewEventID(),
	}
}

// paymentRefundedEvent 组装 payment_refunded 事件。
func paymentRefundedEvent(tid int64, orderNo, paymentNo, returnNo, refundNo, amount, channel string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicPaymentRefunded, Type: eventbus.TypePaymentRefunded,
		Key: orderNo, TenantID: tid,
		Payload: paymentRefundedPayload{
			TenantID: tid, OrderNo: orderNo, PaymentNo: paymentNo,
			ReturnNo: returnNo, RefundNo: refundNo, Amount: amount, Channel: channel,
		},
		EventID: eventbus.NewEventID(),
	}
}
