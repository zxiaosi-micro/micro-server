// contract 业务错误码（contract 段 60000：交易域按 10000 步长续接）。

package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegSegContract 段 = micro-common 预注册段（S5 交易域，勿再 Register）。
const SegContract = errcode.SegContract

var (
	errTemplateNotFound  = errcode.New(SegContract, 1001, "模板不存在")
	errTemplateCodeUsed  = errcode.New(SegContract, 1002, "模板编码已存在")
	errTemplateBodyEmpty = errcode.New(SegContract, 1003, "模板正文必填")
	errContractNotFound  = errcode.New(SegContract, 1004, "合同不存在")
	errContractNoUsed    = errcode.New(SegContract, 1005, "合同号冲突(重试)")
	errContractStatus    = errcode.New(SegContract, 1006, "合同状态不允许该操作")
	errFileRequired      = errcode.New(SegContract, 1007, "生效前须至少一份归档件")
	errFileIdRequired    = errcode.New(SegContract, 1008, "归档件 file_id 必填(file 服务已上传)")
	errWarrantyNotFound  = errcode.New(SegContract, 1009, "质保记录不存在")
	errWarrantyStarted   = errcode.New(SegContract, 1010, "质保已起算(事件重复)")
	errOrderNoRequired   = errcode.New(SegContract, 1011, "来源订单号必填")
	errTargetBad         = errcode.New(SegContract, 1012, "质保对象不合法")
	errStrategyNotFound  = errcode.New(SegContract, 1013, "SLA 策略不存在")
	errStrategyCodeUsed  = errcode.New(SegContract, 1014, "SLA 策略编码已存在")
	errClaimNotFound     = errcode.New(SegContract, 1015, "索赔单不存在")
	errClaimStatus       = errcode.New(SegContract, 1016, "索赔单状态不允许该操作")
	errExtensionNotFound = errcode.New(SegContract, 1017, "延保单不存在")
	errExtensionStatus   = errcode.New(SegContract, 1018, "延保单状态不允许该操作")
	errRenderDisabled    = errcode.New(SegContract, 1019, "打印稿生成已关闭(confcenter)")
	errRenderVarsBad     = errcode.New(SegContract, 1020, "模板变量填充失败(缺必填变量)")
	errMonthsBad         = errcode.New(SegContract, 1021, "月数必须大于 0")
	errAmountBad         = errcode.New(SegContract, 1022, "金额不合法")
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

