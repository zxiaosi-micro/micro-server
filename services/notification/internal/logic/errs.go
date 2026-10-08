// notification 业务错误码（notification 段 140000：平台基础服务按 10000 步长续接）。

package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegNotification 消息服务错误码段。
const SegNotification = errcode.Segment(140000)

// 注册先于错误码 var 求值（Go 包级 var 按依赖+声明序初始化）。
var _ = registerSeg() // 段登记须先于错误码 var 求值（blank 赋值保持初始化序）

var (
	errTemplateUsed  = errcode.New(SegNotification, 1002, "模板编码已存在")
	errTemplateBad   = errcode.New(SegNotification, 1004, "模板必填字段缺失")
	errUserRequired  = errcode.New(SegNotification, 1005, "user_id 必填")
	errQuietHoursBad = errcode.New(SegNotification, 1006, "免打扰时段格式不合法")
	errQuietInEffect = errcode.New(SegNotification, 1007, "当前处于免打扰时段,消息已过滤")
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

// registerSeg 段登记（Register 无返回值，包一层供 var 求值序使用）。
func registerSeg() bool {
	errcode.Register(SegNotification, "notification")
	return true
}
