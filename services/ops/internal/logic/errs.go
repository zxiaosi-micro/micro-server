// ops 业务错误码（ops 段 90000：micro-common 预注册）。
package logic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"

	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegOps 段 = micro-common 预注册段（S7 运维，勿再 Register）。
const SegOps = errcode.SegOps

var (
	errAlertNotFound    = errcode.New(SegOps, 1001, "告警不存在")
	errAlertStatusBad   = errcode.New(SegOps, 1002, "告警状态不允许该操作")
	errAlertCloseNeed   = errcode.New(SegOps, 1003, "关闭前须关联已验收工单或填写原因")
	errRuleNotFound     = errcode.New(SegOps, 1004, "告警规则不存在")
	errRuleCodeDup      = errcode.New(SegOps, 1005, "规则编码已存在")
	errRuleBad          = errcode.New(SegOps, 1006, "规则参数非法（comparator/metric）")
	errTicketNotFound   = errcode.New(SegOps, 1007, "工单不存在")
	errTicketStatusBad  = errcode.New(SegOps, 1008, "工单状态不允许该操作")
	errTicketNoStaff    = errcode.New(SegOps, 1009, "无匹配派单候选（区域+技能）")
	errTicketTypeBad    = errcode.New(SegOps, 1010, "工单类型非法")
	errSlaNotFound      = errcode.New(SegOps, 1011, "SLA 计时器不存在")
	errSlaNotPaused     = errcode.New(SegOps, 1012, "SLA 计时器不在暂停中")
	errSlaPaused        = errcode.New(SegOps, 1013, "SLA 计时器已暂停")
	errSlaPausedNeed    = errcode.New(SegOps, 1014, "暂停原因必填")
	errInspectNotFound  = errcode.New(SegOps, 1015, "巡检任务/计划不存在")
	errInspectSubmitted = errcode.New(SegOps, 1016, "巡检任务已提交")
	errInspectCronBad   = errcode.New(SegOps, 1017, "巡检计划表达式非法（@daily/@weekly/@monthly/@h:N）")
	errItemsRequired    = errcode.New(SegOps, 1018, "巡检明细不能为空")
	errAssigneeRequired = errcode.New(SegOps, 1019, "派单目标 uid 必填（无候选时手动指定）")
)

// mustTenant 业务面租户（BFF authz 注入；事件消费侧由信封恢复）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}

// ---- sql.Null 快捷构造（kernel 组装 model 行用）----

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func sqlNullString(s string) sql.NullString { return nullString(s) }

func nullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func nullFloat(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: true}
}

// validComparator 比较器白名单（与 iotingest rules.hit 同集）。
func validComparator(c string) bool {
	switch c {
	case "gt", "gte", "lt", "lte":
		return true
	}
	return false
}

// isDupKey MySQL 1062 唯一冲突判定。
func isDupKey(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// clampPage 分页钳制。
func clampPage(page, size int64) (int, int) {
	p := int(page)
	s := int(size)
	if p < 1 {
		p = 1
	}
	if s < 1 {
		s = 20
	}
	if s > 100 {
		s = 100
	}
	return p, s
}
