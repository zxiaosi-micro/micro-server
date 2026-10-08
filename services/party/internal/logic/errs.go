// party 业务错误码（SegParty 段内偏移 1000+）。

package logic

import "github.com/zxiaosi-micro/micro-common/errcode"

var (
	errPartyNotFound  = errcode.New(errcode.SegParty, 1001, "参与方不存在")
	errPartyTypeBad   = errcode.New(errcode.SegParty, 1002, "参与方类型不合法")
	errCreditCodeUsed = errcode.New(errcode.SegParty, 1003, "统一社会信用代码已存在")
	errStageBad       = errcode.New(errcode.SegParty, 1005, "商机阶段不合法")
	errAmountBad      = errcode.New(errcode.SegParty, 1006, "金额格式不合法")
	errStaffTypeBad   = errcode.New(errcode.SegParty, 1007, "员工类型不合法")
)
