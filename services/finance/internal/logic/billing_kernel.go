// 发票（FR-FIN-004：ISSUED/REVERSED 状态机桩，生产对接税控服务商）+ 对账任务（FR-FIN-003）。

package logic

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

// ---- 发票 ----

// IssueInvoiceInternal 开票（支付单须已支付；状态机桩——生产对接税控）。
func IssueInvoiceInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	paymentNo, title, taxNo, amount string, uid int64) (string, error) {

	p, err := sc.Models.Payment.FindOneByNo(ctx, tid, paymentNo)
	if err != nil {
		if err == model.ErrNotFound {
			return "", errPaymentNotFound
		}
		return "", err
	}
	if p.Status != "PAID" && p.Status != "SETTLED" {
		return "", errPaymentStatus.WithMsg("未支付不可开票: " + p.Status)
	}
	amountCents := floatToCents(p.Amount)
	if amount != "" {
		if amountCents, err = parseCents(amount); err != nil {
			return "", errAmountBad
		}
	}
	if title == "" {
		return "", errAmountBad.WithMsg("发票抬头必填")
	}

	invoiceNo := "INV" + fmt.Sprint(sc.Snowflake.MustNextID())
	inv := &model.Invoice{
		InvoiceId: sc.Snowflake.MustNextID(), InvoiceNo: invoiceNo,
		PaymentNo: paymentNo, OrderNo: p.OrderNo,
		Title: title, Amount: float64(amountCents) / 100, Status: "ISSUED",
		TenantId: tid, CreatedBy: sqlInt64(uid),
	}
	if taxNo != "" {
		inv.TaxNo = sqlString(taxNo)
	}
	if err := sc.Models.Invoice.InsertTx(ctx, inv); err != nil {
		if isDupKey(err) {
			return "", errInvoiceStatus.WithMsg("发票号冲突(重试)")
		}
		return "", err
	}
	logx.WithContext(ctx).Infof("发票已开 invoice_no=%s payment_no=%s by=%d", invoiceNo, paymentNo, ctxkit.UID(ctx))
	return invoiceNo, nil
}

// ReverseInvoiceInternal 红冲（ISSUED → REVERSED）。
func ReverseInvoiceInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	invoiceNo, reason string, uid int64) error {

	inv, err := sc.Models.Invoice.FindOneByNo(ctx, tid, invoiceNo)
	if err != nil {
		if err == model.ErrNotFound {
			return errInvoiceNotFound
		}
		return err
	}
	if inv.Status != "ISSUED" {
		return errInvoiceStatus.WithMsg("当前状态: " + inv.Status)
	}
	res, err := sc.Conn.ExecCtx(ctx,
		"update `invoice` set `status` = 'REVERSED', `reverse_reason` = ?, `reversed_at` = now(3), `updated_by` = ? where `invoice_id` = ? and `tenant_id` = ? and `status` = 'ISSUED'",
		reason, uid, inv.InvoiceId, tid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errInvoiceStatus
	}
	logx.WithContext(ctx).Infof("发票已红冲 invoice_no=%s by=%d", invoiceNo, ctxkit.UID(ctx))
	return nil
}

// reconcileTaskCols 对账任务显式列（行内查询用）。
const reconcileTaskCols = "`task_id`, `task_no`, `type`, `biz_date`, `diff_report`, `status`, `resolution`, `resolve_remark`, `resolved_by`, `resolved_at`, `tenant_id`, `created_at`, `created_by`, `updated_at`, `updated_by`, `deleted_at`"

// ---- 对账任务 ----

// CreateReconcileTaskInternal 差异任务（同 type+biz_date 唯一——重复报告幂等）。
func CreateReconcileTaskInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	taskType, bizDate, diffReport string) (int64, error) {

	if taskType != "PAYMENT" && taskType != "REFUND" {
		return 0, errTaskStatus.WithMsg("任务类型不合法(PAYMENT/REFUND)")
	}
	if bizDate == "" {
		bizDate = time.Now().Format("2006-01-02")
	}
	taskNo := "RCT" + fmt.Sprint(sc.Snowflake.MustNextID())
	t := &model.ReconcileTask{
		TaskId: sc.Snowflake.MustNextID(), TaskNo: taskNo,
		Type: taskType, BizDate: bizDate, Status: "OPEN",
		TenantId: tid, CreatedBy: sqlInt64(0),
	}
	if diffReport != "" {
		t.DiffReport = sqlString(diffReport)
	}
	if err := sc.Models.ReconcileTask.InsertTx(ctx, t); err != nil {
		if isDupKey(err) {
			// 同日同类型任务已存在：取现有
			var existing model.ReconcileTask
			qerr := sc.Conn.QueryRowCtx(ctx, &existing,
				"select "+reconcileTaskCols+" from `reconcile_task` where `type` = ? and `biz_date` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
				taskType, bizDate, tid)
			if qerr == nil {
				return existing.TaskId, nil
			}
			return 0, err
		}
		return 0, err
	}
	logx.WithContext(ctx).Infof("对账任务已建 task_no=%s type=%s date=%s", taskNo, taskType, bizDate)
	return t.TaskId, nil
}

// ResolveReconcileTaskInternal 差异处理（修复只允许补投递重放 REPLAY / IGNORE，E10——禁裸删）。
func ResolveReconcileTaskInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	taskId int64, resolution, remark string, uid int64) error {

	if resolution != "REPLAY" && resolution != "IGNORE" {
		return errResolutionBad
	}
	res, err := sc.Conn.ExecCtx(ctx,
		"update `reconcile_task` set `status` = 'RESOLVED', `resolution` = ?, `resolve_remark` = ?, `resolved_by` = ?, `resolved_at` = now(3) where `task_id` = ? and `tenant_id` = ? and `status` = 'OPEN'",
		resolution, remark, uid, taskId, tid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var t model.ReconcileTask
		if err := sc.Conn.QueryRowCtx(ctx, &t, "select "+reconcileTaskCols+" from `reconcile_task` where `task_id` = ? and `tenant_id` = ? and `deleted_at` is null", taskId, tid); err != nil {
			return errTaskNotFound
		}
		return errTaskStatus.WithMsg("任务已处理: " + t.Status)
	}
	logx.WithContext(ctx).Infof("对账任务已处理 task_id=%d resolution=%s by=%d", taskId, resolution, ctxkit.UID(ctx))
	return nil
}
