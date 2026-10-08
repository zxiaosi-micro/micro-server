// finance 事件构造（S5-01 契约）：topic 下划线 / event_type 点号 / key=order_no（保序聚合键）。

package logic

import "github.com/zxiaosi-micro/micro-common/eventbus"

// orderPaidPayload order_paid 事件体（order 消费 → Saga 步骤 3→4 推进）。
type orderPaidPayload struct {
	TenantID   int64  `json:"tenant_id"`
	OrderNo    string `json:"order_no"`
	PaymentNo  string `json:"payment_no"`
	Channel    string `json:"channel"`
	PaidAmount string `json:"paid_amount"`
	PaidAt     int64  `json:"paid_at"` // UnixMilli
}

// paymentRefundedPayload payment_refunded 事件体（order 消费 → 退货单 REFUNDED 回写）。
type paymentRefundedPayload struct {
	TenantID int64  `json:"tenant_id"`
	OrderNo  string `json:"order_no"`
	PaymentNo string `json:"payment_no"`
	ReturnNo string `json:"return_no"` // 空=手工退款
	RefundNo string `json:"refund_no"`
	Amount   string `json:"amount"`
	Channel  string `json:"channel"`
}

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
