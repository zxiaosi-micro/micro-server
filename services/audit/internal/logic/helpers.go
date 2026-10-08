// audit 业务错误码（audit 段 130000：平台基础服务按 10000 步长续接登记）+ 公共助手。

package logic

import (
	"context"
	"encoding/json"

	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegAudit 审计服务错误码段。
const SegAudit = errcode.Segment(130000)

// 注册先于错误码 var 求值（同 errs.go 说明）。
var _ = registerSeg() // 段登记须先于错误码 var 求值（blank 赋值保持初始化序）

var (
	errActionRequired = errcode.New(SegAudit, 1001, "action 必填")
	errCmdRequired    = errcode.New(SegAudit, 1002, "cmd_id/sn 必填")
)

// auditTenant 租户来源：显式入参 > ctx（后台作业走入参；BFF 链路走 ctx）。
func auditTenant(ctx context.Context, explicit int64) (int64, error) {
	if explicit > 0 {
		return explicit, nil
	}
	if tid, ok := tenantx.TenantFromCtx(ctx); ok {
		return tid, nil
	}
	return 0, nil // 平台级审计允许无租户（系统动作 uid=0）
}

// validJSON 校验入参 JSON 字段。
func validJSON(s string) bool {
	if s == "" {
		return true
	}
	return json.Valid([]byte(s))
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

// registerSeg 段登记（Register 无返回值，包一层供 var 求值序使用）。
func registerSeg() bool {
	errcode.Register(SegAudit, "audit")
	return true
}
