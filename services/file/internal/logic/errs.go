// file 业务错误码（file 段 120000：平台基础服务按 10000 步长续接登记，02 §8）。

package logic

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegFile file 段=120000（与 micro-common errcode registeredSegments 对账）。
// 注意：errcode.New 在包级 var 求值时即校验段——注册必须先于错误码 var 声明执行
// （Go 包级 var 按依赖+声明序初始化，此处用 var 触发注册而非 init()）。
const SegFile = errcode.Segment(120000)

var _ = registerSeg() // 段登记须先于错误码 var 求值（blank 赋值保持初始化序）

var (
	errFileNotFound = errcode.New(SegFile, 1001, "文件不存在")
	errFileTooLarge = errcode.New(SegFile, 1002, "文件超出大小限制")
	errTokenInvalid = errcode.New(SegFile, 1003, "下载令牌无效或已过期")
	errUploadOff    = errcode.New(SegFile, 1004, "上传通道已关闭")
	errOssPut       = errcode.New(SegFile, 1005, "对象存储写入失败")
	errOssGet       = errcode.New(SegFile, 1006, "对象存储读取失败")
)

// mustTenant 业务面租户（BFF authz 注入）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}

// opUID 操作人 uid。
func opUID(ctx context.Context) int64 {
	return ctxkit.UID(ctx)
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

// registerSeg 段登记（Register 无返回值，包一层供 var 求值序使用）。
func registerSeg() bool {
	errcode.Register(SegFile, "file")
	return true
}
