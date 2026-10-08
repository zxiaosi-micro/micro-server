// 事件构造（S5-01 事件契约）：topic 下划线 / event_type 点号 / key=聚合 ID / Outbox 同事务 Emit。
// 消费方契约注释写在各 payload 字段——改字段即改契约，须同步消费方（order/finance/contract/ops/notification）。

package logic

import (
	"encoding/json"
	"math"

	"micro-server/services/order/internal/model"

	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// orderCreatedPayload order_created 事件体（notification 欢迎语/audit 留痕消费）。
type orderCreatedPayload struct {
	TenantID     int64  `json:"tenant_id"`
	OrderID      int64  `json:"order_id"`
	OrderNo      string `json:"order_no"`
	OrderType    string `json:"order_type"`
	TotalAmount  string `json:"total_amount"`
	BuyerPartyID int64  `json:"buyer_party_id"`
}

// orderCancelledPayload order_cancelled 事件体（库存已释放/未锁，inventory 无需动作）。
type orderCancelledPayload struct {
	TenantID    int64  `json:"tenant_id"`
	OrderNo     string `json:"order_no"`
	Reason      string `json:"reason"`
	CancelledBy string `json:"cancelled_by"` // USER/PAY_TIMEOUT
}

// orderPayTimeoutPayload order_pay_timeout 事件体（ADR-09：pay_expire_at 扫描产生，无 MQ 延迟消息）。
type orderPayTimeoutPayload struct {
	TenantID  int64  `json:"tenant_id"`
	OrderNo   string `json:"order_no"`
	ExpireAt  int64  `json:"expire_at"` // UnixMilli
}

// orderReturnApprovedPayload order_return_approved 事件体（finance 消费 → CreateRefund 原路退回）。
type orderReturnApprovedPayload struct {
	TenantID     int64  `json:"tenant_id"`
	ReturnNo     string `json:"return_no"`
	OrderNo      string `json:"order_no"`
	RefundAmount string `json:"refund_amount"` // DECIMAL（按退货明细 × 单价快照计）
	Reason       string `json:"reason"`
}

// shipmentSignedPayload shipment_signed 事件体（contract 质保起算 + S6 设备域出库状态消费）。
type shipmentSignedPayload struct {
	TenantID   int64    `json:"tenant_id"`
	OrderNo    string   `json:"order_no"`
	ShipmentNo string   `json:"shipment_no"`
	Sns        []string `json:"sns"`       // 签收设备 SN 清单（可空：无 SN 的普通货物）
	SignedBy   string   `json:"signed_by"`
	SignedAt   int64    `json:"signed_at"` // UnixMilli
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func orderCreatedEvent(tid int64, o *model.Order) eventbus.EmitInput {
	p := orderCreatedPayload{
		TenantID: tid, OrderID: o.OrderId, OrderNo: o.OrderNo, OrderType: o.Type,
		TotalAmount: centsToAmount(int64(math.Round(o.TotalAmount * 100))), BuyerPartyID: o.BuyerPartyId.Int64,
	}
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderCreated, Type: eventbus.TypeOrderCreated,
		Key: o.OrderNo, TenantID: tid, Payload: p, EventID: eventbus.NewEventID(),
	}
}

func orderCancelledEvent(tid int64, orderNo, reason, cancelledBy string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderCancelled, Type: eventbus.TypeOrderCancelled,
		Key: orderNo, TenantID: tid,
		Payload: orderCancelledPayload{TenantID: tid, OrderNo: orderNo, Reason: reason, CancelledBy: cancelledBy},
		EventID: eventbus.NewEventID(),
	}
}

func orderPayTimeoutEvent(tid int64, orderNo string, expireAtMilli int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderPayTimeout, Type: eventbus.TypeOrderPayTimeout,
		Key: orderNo, TenantID: tid,
		Payload: orderPayTimeoutPayload{TenantID: tid, OrderNo: orderNo, ExpireAt: expireAtMilli},
		EventID: eventbus.NewEventID(),
	}
}

func orderReturnApprovedEvent(tid int64, returnNo, orderNo, amount, reason string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderReturnApproved, Type: eventbus.TypeOrderReturnApproved,
		Key: returnNo, TenantID: tid,
		Payload: orderReturnApprovedPayload{TenantID: tid, ReturnNo: returnNo, OrderNo: orderNo, RefundAmount: amount, Reason: reason},
		EventID: eventbus.NewEventID(),
	}
}

func shipmentSignedEvent(tid int64, orderNo, shipmentNo string, sns []string, signedBy string, signedAtMilli int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicShipmentSigned, Type: eventbus.TypeShipmentSigned,
		Key: orderNo, TenantID: tid,
		Payload: shipmentSignedPayload{TenantID: tid, OrderNo: orderNo, ShipmentNo: shipmentNo, Sns: sns, SignedBy: signedBy, SignedAt: signedAtMilli},
		EventID: eventbus.NewEventID(),
	}
}
