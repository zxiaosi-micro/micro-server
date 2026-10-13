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
	TenantID     int64  `json:"tenant_id"`      // 租户 ID（与信封同源冗余，消费侧免解析信封）
	OrderID      int64  `json:"order_id"`       // 订单 ID（雪花）
	OrderNo      string `json:"order_no"`       // 订单号（消费侧幂等键）
	OrderType    string `json:"order_type"`     // DEVICE/STATION/PURCHASE/RETURN
	TotalAmount  string `json:"total_amount"`   // 订单总额（DECIMAL 元字符串，运算走分）
	BuyerPartyID int64  `json:"buyer_party_id"` // 买方参与方 ID（0=未挂参与方）
}

// orderCancelledPayload order_cancelled 事件体（库存已释放/未锁，inventory 无需动作）。
type orderCancelledPayload struct {
	TenantID    int64  `json:"tenant_id"`    // 租户 ID
	OrderNo     string `json:"order_no"`     // 订单号（消费侧幂等键）
	Reason      string `json:"reason"`       // 取消原因
	CancelledBy string `json:"cancelled_by"` // USER/PAY_TIMEOUT
}

// orderPayTimeoutPayload order_pay_timeout 事件体（ADR-09：pay_expire_at 扫描产生，无 MQ 延迟消息）。
type orderPayTimeoutPayload struct {
	TenantID int64  `json:"tenant_id"` // 租户 ID
	OrderNo  string `json:"order_no"`  // 订单号（消费侧幂等键）
	ExpireAt int64  `json:"expire_at"` // 支付截止时刻（UnixMilli）
}

// orderReturnApprovedPayload order_return_approved 事件体（finance 消费 → CreateRefund 原路退回）。
type orderReturnApprovedPayload struct {
	TenantID     int64  `json:"tenant_id"`     // 租户 ID
	ReturnNo     string `json:"return_no"`     // 退货单号（消费侧幂等键）
	OrderNo      string `json:"order_no"`      // 原订单号
	RefundAmount string `json:"refund_amount"` // 退款金额（DECIMAL，按退货明细 × 单价快照计）
	Reason       string `json:"reason"`        // 退货原因
}

// shipmentSignedPayload shipment_signed 事件体（contract 质保起算 + S6 设备域出库状态消费）。
type shipmentSignedPayload struct {
	TenantID   int64    `json:"tenant_id"`   // 租户 ID
	OrderNo    string   `json:"order_no"`    // 订单号
	ShipmentNo string   `json:"shipment_no"` // 发货单号（消费侧幂等键）
	Sns        []string `json:"sns"`         // 签收设备 SN 清单（可空：无 SN 的普通货物）
	SignedBy   string   `json:"signed_by"`   // 签收人
	SignedAt   int64    `json:"signed_at"`   // 签收时刻（UnixMilli，质保起算时间源）
}

// mustJSON 序列化事件负载（失败返回 "{}"，事件总线不因负载问题阻断业务事务）。
func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// orderCreatedEvent 组装 order_created 事件（key=order_no 同聚合同分区保序）。
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

// orderCancelledEvent 组装 order_cancelled 事件。
func orderCancelledEvent(tid int64, orderNo, reason, cancelledBy string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderCancelled, Type: eventbus.TypeOrderCancelled,
		Key: orderNo, TenantID: tid,
		Payload: orderCancelledPayload{TenantID: tid, OrderNo: orderNo, Reason: reason, CancelledBy: cancelledBy},
		EventID: eventbus.NewEventID(),
	}
}

// orderPayTimeoutEvent 组装 order_pay_timeout 事件（支付超时 cron 扫描产生）。
func orderPayTimeoutEvent(tid int64, orderNo string, expireAtMilli int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderPayTimeout, Type: eventbus.TypeOrderPayTimeout,
		Key: orderNo, TenantID: tid,
		Payload: orderPayTimeoutPayload{TenantID: tid, OrderNo: orderNo, ExpireAt: expireAtMilli},
		EventID: eventbus.NewEventID(),
	}
}

// orderReturnApprovedEvent 组装 order_return_approved 事件。
func orderReturnApprovedEvent(tid int64, returnNo, orderNo, amount, reason string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicOrderReturnApproved, Type: eventbus.TypeOrderReturnApproved,
		Key: returnNo, TenantID: tid,
		Payload: orderReturnApprovedPayload{TenantID: tid, ReturnNo: returnNo, OrderNo: orderNo, RefundAmount: amount, Reason: reason},
		EventID: eventbus.NewEventID(),
	}
}

// shipmentSignedEvent 组装 shipment_signed 事件（质保起算触发源）。
func shipmentSignedEvent(tid int64, orderNo, shipmentNo string, sns []string, signedBy string, signedAtMilli int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic: eventbus.TopicShipmentSigned, Type: eventbus.TypeShipmentSigned,
		Key: orderNo, TenantID: tid,
		Payload: shipmentSignedPayload{TenantID: tid, OrderNo: orderNo, ShipmentNo: shipmentNo, Sns: sns, SignedBy: signedBy, SignedAt: signedAtMilli},
		EventID: eventbus.NewEventID(),
	}
}
