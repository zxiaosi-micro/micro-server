// 支付单内核（S5-03，FR-FIN-001/002）：
//
//	CREATE：三渠道受理——WECHAT/ALIPAY → PAYING（渠道 URL），BANK_OFFLINE → REVIEWING（对公待复核）；
//	CONFIRM：渠道回调/模拟网关确认 → PAID + channel_txn_id（UK 幂等）+ order_paid 事件；
//	APPROVE：对公第二人复核（职责分离：复核人 ≠ 录入人）→ PAID/REJECTED；
//	金额不符：按实收落账 + 差异对账任务（FR-FIN-003）。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"micro-server/services/finance/internal/confcenter"
	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// orderPaidEmit 支付完成事件（业务事务内 outbox；finance 是 order_paid 的生产方）。
func orderPaidEmit(ctx context.Context, session sqlx.Session, tid int64, p *model.Payment) error {
	paidAt := p.PaidAt.Time.UnixMilli()
	if !p.PaidAt.Valid {
		paidAt = time.Now().UnixMilli()
	}
	return eventbus.Emit(ctx, session, orderPaidEvent(tid, p.OrderNo, p.PaymentNo, p.Channel,
		centsToAmount(floatToCents(p.PaidAmount.Float64)), paidAt))
}

// CreatePayment 建支付单（PayOrder 幂等锚点：同单活跃单直接返回）。
func CreatePayment(ctx context.Context, sc *svc.ServiceContext, tid int64,
	orderNo, amount, channel string, payerPartyId int64, remark string, uid int64) (string, string, string, error) {

	cc := confcenter.Current()
	if channel != "WECHAT" && channel != "ALIPAY" && channel != "BANK_OFFLINE" {
		return "", "", "", errChannelBad
	}
	// 渠道降级开关（02 §10 降级行为：只收对公）
	if channel != "BANK_OFFLINE" && cc.ChannelDegraded {
		return "", "", "", errChannelDisabled
	}
	if channel == "WECHAT" && !cc.WechatEnabled {
		return "", "", "", errChannelDisabled.WithMsg("微信渠道已关闭")
	}
	if channel == "ALIPAY" && !cc.AlipayEnabled {
		return "", "", "", errChannelDisabled.WithMsg("支付宝渠道已关闭")
	}
	amountCents, err := parseCents(amount)
	if err != nil || amountCents <= 0 {
		return "", "", "", errAmountBad
	}

	// 幂等：活跃单存在 → 直接返回
	if active, err := sc.Models.Payment.FindActiveByOrder(ctx, tid, orderNo); err == nil {
		return active.PaymentNo, active.Status, nullStr(active.PayParams), nil
	} else if err != model.ErrNotFound {
		return "", "", "", err
	}

	status := "PAYING"
	if channel == "BANK_OFFLINE" {
		status = "REVIEWING" // 对公录入 → 第二人复核（REVIEWING 中间态）
	}

	payNo := "PAY" + fmt.Sprint(sc.Snowflake.MustNextID())
	payParams := ""
	if channel == "WECHAT" || channel == "ALIPAY" {
		payParams = sc.Gateway.MockPayURL(payNo) // 真实渠道 URL 由 PayURL RPC 二次取（配置驱动）
	}

	p := &model.Payment{
		PaymentId: sc.Snowflake.MustNextID(), PaymentNo: payNo,
		OrderNo: orderNo, Channel: channel, Status: status,
		Amount: float64(amountCents) / 100,
		TenantId: tid, CreatedBy: sqlInt64(uid),
	}
	if payerPartyId > 0 {
		p.PayerPartyId = sqlInt64(payerPartyId)
	}
	if remark != "" {
		p.Remark = sqlString(remark)
	}
	if payParams != "" {
		p.PayParams = sqlString(payParams)
	}
	if err := sc.Models.Payment.InsertTx(ctx, sc.Conn, p); err != nil {
		return "", "", "", err
	}
	logx.WithContext(ctx).Infof("支付单已建 payment_no=%s order_no=%s channel=%s status=%s", payNo, orderNo, channel, status)
	return payNo, status, payParams, nil
}

// ConfirmPayment 支付确认（渠道回调验签后 / dev 模拟网关）。
// 金额不符：按实收落账 + 差异对账任务；channel_txn_id UK 1062 → 幂等重放。
func ConfirmPayment(ctx context.Context, sc *svc.ServiceContext, tid int64,
	paymentNo, channelTxnId, paidAmount, source string) (string, error) {

	var amountMismatch string
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		p, err := sc.Models.Payment.FindOneByNoForUpdateTx(ctx, session, tid, paymentNo)
		if err != nil {
			return err
		}
		if p.Status == "PAID" || p.Status == "SETTLED" {
			return nil // 幂等：已确认
		}
		if p.Channel == "BANK_OFFLINE" && source != "MOCK" {
			return errPaymentStatus.WithMsg("对公支付须经第二人复核(ApprovePayment)")
		}
		paidCents, err := parseCents(paidAmount)
		if err != nil {
			return errAmountBad
		}
		wantCents := floatToCents(p.Amount)
		if paidCents != wantCents {
			// 金额不符：按实收落账 + 差异任务（FR-FIN-003）
			amountMismatch = fmt.Sprintf(`{"payment_no":"%s","want":"%s","paid":"%s"}`,
				paymentNo, centsToAmount(wantCents), centsToAmount(paidCents))
			logx.WithContext(ctx).Errorf("支付金额不符 payment_no=%s want=%s paid=%s", paymentNo,
				centsToAmount(wantCents), centsToAmount(paidCents))
		}
		paidAt := time.Now()
		if err := sc.Models.Payment.MarkPaidTx(ctx, session, tid, p.PaymentId, channelTxnId, centsToAmount(paidCents), paidAt); err != nil {
			return err
		}
		return orderPaidEmit(ctx, session, tid, &model.Payment{
			PaymentNo: p.PaymentNo, OrderNo: p.OrderNo, Channel: p.Channel,
			PaidAt: sqlTime(paidAt), PaidAmount: sqlFloat(paidCents),
		})
	})
	if err != nil {
		if err == model.ErrNotFound {
			return "", errPaymentNotFound
		}
		if isDupKey(err) {
			return "PAID", nil // 渠道流水重复（重放）：幂等成功
		}
		return "", err
	}
	if amountMismatch != "" {
		if _, err := CreateReconcileTaskInternal(ctx, sc, tid, "PAYMENT", time.Now().Format("2006-01-02"), amountMismatch); err != nil {
			logx.WithContext(ctx).Errorf("差异对账任务创建失败 payment_no=%s: %v", paymentNo, err)
		}
	}
	logx.WithContext(ctx).Infof("支付已确认 payment_no=%s source=%s", paymentNo, source)
	return "PAID", nil
}

// ApprovePayment 对公第二人复核（职责分离：复核人 ≠ 录入人，FR-FIN-001）。
func ApprovePayment(ctx context.Context, sc *svc.ServiceContext, tid int64,
	paymentNo string, approve bool, remark string, uid int64) error {

	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		p, err := sc.Models.Payment.FindOneByNoForUpdateTx(ctx, session, tid, paymentNo)
		if err != nil {
			return err
		}
		if p.Status != "REVIEWING" {
			return errPaymentStatus.WithMsg("当前状态: " + p.Status)
		}
		if p.CreatedBy.Valid && p.CreatedBy.Int64 == uid {
			return errSelfApproval // 职责分离硬约束
		}
		if err := sc.Models.Payment.MarkReviewedTx(ctx, session, tid, p.PaymentId, uid, approve, remark); err != nil {
			return err
		}
		if approve {
			return orderPaidEmit(ctx, session, tid, &model.Payment{
				PaymentNo: p.PaymentNo, OrderNo: p.OrderNo, Channel: p.Channel,
				PaidAt: sqlTime(time.Now()), PaidAmount: sqlFloat(floatToCents(p.Amount)),
			})
		}
		return nil
	})
	if err != nil {
		if err == model.ErrNotFound {
			return errPaymentNotFound
		}
		return err
	}
	logx.WithContext(ctx).Infof("对公复核完成 payment_no=%s approve=%v by=%d", paymentNo, approve, uid)
	return nil
}

// SettlePayment 对账核销（PAID → SETTLED）。
func SettlePayment(ctx context.Context, sc *svc.ServiceContext, tid int64, paymentNo, remark string) error {
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		p, err := sc.Models.Payment.FindOneByNoForUpdateTx(ctx, session, tid, paymentNo)
		if err != nil {
			return err
		}
		if err := sc.Models.Payment.CASStatusTx(ctx, session, tid, p.PaymentId, []string{"PAID"}, "SETTLED"); err != nil {
			return err
		}
		_ = remark
		return nil
	})
	if err != nil {
		if err == model.ErrNotFound {
			return errPaymentNotFound
		}
		if err == model.ErrStatusConflict {
			return errPaymentStatus.WithMsg("仅 PAID 可核销")
		}
		return err
	}
	return nil
}

var _ = math.Round
var _ = json.Marshal
