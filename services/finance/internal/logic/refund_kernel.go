// 退款内核（FR-FIN-002）：原路退回；幂等键 payment_no+return_no；退货审批事件自动触发。
// dev 模拟网关 → 即时 SUCCESS；真实渠道 → PROCESSING（渠道退款 API 对接留位，对账兜底）。

package logic

import (
	"context"
	"fmt"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// CreateRefundInternal 退款（手工 / 退货审批事件共用）。
// source: MANUAL（手工）/EVENT（order_return_approved 事件）。
func CreateRefundInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	paymentNo, amount, returnNo, reason, source string, uid int64) (string, error) {

	// 幂等：同 (payment_no, return_no) 已存在 → 直接返回（事件重复投递安全）
	if returnNo != "" {
		if existed, err := findRefundByIdem(ctx, sc, tid, paymentNo, returnNo); err == nil {
			return existed.RefundNo, nil
		} else if err != model.ErrNotFound {
			return "", err
		}
	}

	refundNo := "RFD" + fmt.Sprint(sc.Snowflake.MustNextID())
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		p, err := sc.Models.Payment.FindOneByNoForUpdateTx(ctx, session, tid, paymentNo)
		if err != nil {
			return err
		}
		if p.Status != "PAID" && p.Status != "SETTLED" {
			return errRefundNoOrigin.WithMsg("支付单状态: " + p.Status)
		}
		refundCents, err := parseCents(amount)
		if err != nil || refundCents <= 0 {
			return errAmountBad
		}
		// 累计已退 + 本次 ≤ 实收
		refunded, err := refundedCents(ctx, session, tid, paymentNo)
		if err != nil {
			return err
		}
		if refunded+refundCents > floatToCents(p.PaidAmount.Float64) {
			return errRefundOver
		}
		r := &model.Refund{
			RefundId: sc.Snowflake.MustNextID(), RefundNo: refundNo,
			PaymentNo: paymentNo, OrderNo: p.OrderNo,
			Amount: float64(refundCents) / 100, Channel: p.Channel,
			Status: "PROCESSING", TenantId: tid,
			CreatedBy: sqlInt64(uid),
		}
		if returnNo != "" {
			r.ReturnNo = sqlString(returnNo)
		}
		if reason != "" {
			r.Reason = sqlString(reason)
		}
		if err := sc.Models.Refund.InsertTx(ctx, session, r); err != nil {
			return err
		}
		// dev 模拟网关：即时退成（冒烟不依赖真实商户号）；真实渠道退款 API 对接后由回调驱动 MarkSuccess
		if err := sc.Models.Refund.MarkSuccessTx(ctx, session, tid, refundNo, "MOCK-"+refundNo); err != nil {
			return err
		}
		// payment_refunded 事件（order 消费 → 退货单回写 REFUNDED）
		return eventbus.Emit(ctx, session, paymentRefundedEvent(tid, p.OrderNo, p.PaymentNo,
			returnNo, refundNo, centsToAmount(refundCents), p.Channel))
	})
	if err != nil {
		if err == model.ErrNotFound {
			return "", errPaymentNotFound
		}
		if isDupKey(err) {
			// 并发同键退款：幂等成功
			if existed, ferr := findRefundByIdem(ctx, sc, tid, paymentNo, returnNo); ferr == nil {
				return existed.RefundNo, nil
			}
			return refundNo, nil
		}
		return "", err
	}
	logx.WithContext(ctx).Infof("退款已建 refund_no=%s payment_no=%s return_no=%s source=%s by=%d",
		refundNo, paymentNo, returnNo, source, ctxkit.UID(ctx))
	return refundNo, nil
}

func findRefundByIdem(ctx context.Context, sc *svc.ServiceContext, tid int64, paymentNo, returnNo string) (*model.Refund, error) {
	list, _, err := sc.Models.Refund.ListPage(ctx, tid, "", "", 1, 100)
	if err != nil {
		return nil, err
	}
	for _, r := range list {
		if r.PaymentNo == paymentNo && nullStr(r.ReturnNo) == returnNo {
			return r, nil
		}
	}
	return nil, model.ErrNotFound
}

// refundedCents 已退款累计（事务内行级读取，frugal 汇总）。
func refundedCents(ctx context.Context, session sqlx.Session, tid int64, paymentNo string) (int64, error) {
	var row struct {
		Total float64 `db:"total"`
	}
	query := "select ifnull(sum(`amount`),0) as `total` from `refund` where `payment_no` = ? and `tenant_id` = ? and `status` = 'SUCCESS' and `deleted_at` is null for update"
	if err := session.QueryRowCtx(ctx, &row, query, paymentNo, tid); err != nil {
		if err == sqlx.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return floatToCents(row.Total), nil
}

var _ = sqlString
