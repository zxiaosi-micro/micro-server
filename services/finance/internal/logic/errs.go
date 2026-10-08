// finance 业务错误码（finance 段 100000：交易域按 10000 步长续接）。

package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegSegFinance 段 = micro-common 预注册段（S5 交易域，勿再 Register）。
const SegFinance = errcode.SegFinance

var (
	errPaymentNotFound  = errcode.New(SegFinance, 1001, "支付单不存在")
	errPaymentNoUsed    = errcode.New(SegFinance, 1002, "支付单号已存在")
	errPaymentStatus    = errcode.New(SegFinance, 1003, "支付单状态不允许该操作")
	errChannelBad       = errcode.New(SegFinance, 1004, "支付渠道不合法")
	errChannelDisabled  = errcode.New(SegFinance, 1005, "支付渠道已关闭(降级只收对公)")
	errAmountBad        = errcode.New(SegFinance, 1006, "金额不合法")
	errAmountMismatch   = errcode.New(SegFinance, 1007, "实收金额与应收不符(已生成对账差异)")
	errSelfApproval     = errcode.New(SegFinance, 1008, "复核人不得与录入人相同(职责分离)")
	errRefundNotFound   = errcode.New(SegFinance, 1009, "退款单不存在")
	errRefundNoOrigin   = errcode.New(SegFinance, 1010, "原支付单不可退款(未支付)")
	errRefundOver       = errcode.New(SegFinance, 1011, "退款金额超过可退余额")
	errInvoiceNotFound  = errcode.New(SegFinance, 1012, "发票不存在")
	errInvoiceStatus    = errcode.New(SegFinance, 1013, "发票状态不允许该操作")
	errTaskNotFound     = errcode.New(SegFinance, 1014, "对账任务不存在")
	errTaskStatus       = errcode.New(SegFinance, 1015, "对账任务状态不允许该操作")
	errResolutionBad    = errcode.New(SegFinance, 1016, "处理方式不合法(仅 REPLAY/IGNORE)")
	errChannelVerifyBad = errcode.New(SegFinance, 1017, "回调验签失败")
)

// mustTenant 业务面租户（BFF authz 注入）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}

// isDupKey MySQL 1062 唯一冲突判定。
func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// clampPage 分页上限收敛。
func clampPage(page, size int64) (int64, int64) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

