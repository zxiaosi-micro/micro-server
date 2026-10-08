// 事件消费入口 + 视图组装（finance）。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"

	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// HandleReturnApproved order_return_approved（order 发出）→ 自动 CreateRefund 原路退回（FR-FIN-002）。
func HandleReturnApproved(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			ReturnNo     string `json:"return_no"`
			OrderNo      string `json:"order_no"`
			RefundAmount string `json:"refund_amount"`
			Reason       string `json:"reason"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("order_return_approved 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		log := logx.WithContext(ctx).WithFields(logx.Field("return_no", p.ReturnNo))

		// 找原支付单（已支付/已核销）
		var payment *model.Payment
		list, _, err := sc.Models.Payment.ListPage(ctx, tid, p.OrderNo, "", "", 1, 20)
		if err != nil {
			return err
		}
		for i := range list {
			if list[i].OrderNo == p.OrderNo && (list[i].Status == "PAID" || list[i].Status == "SETTLED") {
				payment = list[i]
				break
			}
		}
		if payment == nil {
			log.Errorf("退货审批事件无可退支付单 order_no=%s（留痕，人工核对）", p.OrderNo)
			return nil // ack：无退款对象，重投无意义
		}

		refundNo, err := CreateRefundInternal(ctx, sc, tid, payment.PaymentNo, p.RefundAmount, p.ReturnNo, p.Reason, "EVENT", 0)
		if err != nil {
			return err // 失败重投（event_retry 退避）
		}
		log.Infof("退货退款已受理 refund_no=%s amount=%s", refundNo, p.RefundAmount)
		return nil
	}
}

// ---- 视图组装 ----

func buildPaymentDetail(p *model.Payment) *finpb.PaymentDetail {
	d := &finpb.PaymentDetail{
		PaymentNo:     p.PaymentNo,
		OrderNo:       p.OrderNo,
		Channel:       p.Channel,
		Status:        p.Status,
		Amount:        centsToAmount(floatToCents(p.Amount)),
		ChannelTxnId:  nullStr(p.ChannelTxnId),
		PayerPartyId:  p.PayerPartyId.Int64,
		CreatedBy:     p.CreatedBy.Int64,
		ApprovedBy:    p.ApprovedBy.Int64,
		Remark:        nullStr(p.Remark),
		CreatedAt:     p.CreatedAt.UnixMilli(),
	}
	if p.PaidAmount.Valid {
		d.PaidAmount = centsToAmount(floatToCents(p.PaidAmount.Float64))
	}
	if p.PaidAt.Valid {
		d.PaidAt = p.PaidAt.Time.UnixMilli()
	}
	return d
}

func buildRefundDetail(r *model.Refund) *finpb.RefundDetail {
	d := &finpb.RefundDetail{
		RefundNo:  r.RefundNo,
		PaymentNo: r.PaymentNo,
		OrderNo:   r.OrderNo,
		ReturnNo:  nullStr(r.ReturnNo),
		Amount:    centsToAmount(floatToCents(r.Amount)),
		Channel:   r.Channel,
		Status:    r.Status,
		Reason:    nullStr(r.Reason),
		CreatedAt: r.CreatedAt.UnixMilli(),
	}
	d.ChannelRefundId = nullStr(r.ChannelRefundId)
	return d
}

func buildInvoiceDetail(inv *model.Invoice) *finpb.InvoiceDetail {
	d := &finpb.InvoiceDetail{
		InvoiceNo:    inv.InvoiceNo,
		PaymentNo:    inv.PaymentNo,
		OrderNo:      inv.OrderNo,
		Title:        inv.Title,
		TaxNo:        nullStr(inv.TaxNo),
		Amount:       centsToAmount(floatToCents(inv.Amount)),
		Status:       inv.Status,
		ReverseReason: nullStr(inv.ReverseReason),
		CreatedAt:    inv.CreatedAt.UnixMilli(),
	}
	if inv.ReversedAt.Valid {
		d.IssuedAt = inv.ReversedAt.Time.UnixMilli()
	} else {
		d.IssuedAt = inv.CreatedAt.UnixMilli()
	}
	return d
}

func buildReconcileTaskDetail(t *model.ReconcileTask) *finpb.ReconcileTaskDetail {
	return &finpb.ReconcileTaskDetail{
		TaskId:        t.TaskId,
		TaskNo:        t.TaskNo,
		Type:          t.Type,
		BizDate:       t.BizDate,
		DiffReport:    nullStr(t.DiffReport),
		Status:        t.Status,
		Resolution:    nullStr(t.Resolution),
		ResolveRemark: nullStr(t.ResolveRemark),
		ResolvedBy:    t.ResolvedBy.Int64,
		CreatedAt:     t.CreatedAt.UnixMilli(),
	}
}

var _ = time.Now
