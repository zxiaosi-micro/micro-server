// analytics 业务错误码（S8：分析域段 150000，平台基础服务按 10000 步长续接登记，02 §8）。

package logic

import (
	"context"

	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegAnalytics 段=150000（与 micro-common errcode registeredSegments 对账）。
// 注意：errcode.New 在包级 var 求值时即校验段——注册必须先于错误码 var 声明执行
// （Go 包级 var 按依赖+声明序初始化，此处用 var 触发注册而非 init()）。
const SegAnalytics = errcode.Segment(150000)

var _ = registerSeg() // 段登记须先于错误码 var 求值（blank 赋值保持初始化序）

var (
	errReportTypeBad   = errcode.New(SegAnalytics, 1001, "报表类型不合法")
	errDateFormatBad   = errcode.New(SegAnalytics, 1002, "日期格式不合法（yyyy-MM-dd）")
	errExportFormatBad = errcode.New(SegAnalytics, 1003, "导出格式不合法（CSV/XLSX/PDF）")
	errExportNotFound  = errcode.New(SegAnalytics, 1004, "导出任务不存在")
	errExportRunning   = errcode.New(SegAnalytics, 1005, "导出任务已在处理中")
	errExportNotDone   = errcode.New(SegAnalytics, 1006, "导出任务尚未完成")
	errPdfFontMissing  = errcode.New(SegAnalytics, 1007, "PDF 字体未配置（pdf_font），PDF 导出不可用")
	errReconDateBad    = errcode.New(SegAnalytics, 1008, "对账日期不合法")
	errReconRunning    = errcode.New(SegAnalytics, 1009, "该业务日对账正在执行")

	// S10-02 BI 自助 + 数仓 ETL。
	errBITableBad   = errcode.New(SegAnalytics, 1010, "查询表不在白名单")
	errBIColumnBad  = errcode.New(SegAnalytics, 1011, "查询列不在白名单")
	errBIOpBad      = errcode.New(SegAnalytics, 1012, "过滤操作符/值不合法")
	errBIAggBad     = errcode.New(SegAnalytics, 1013, "聚合不合法")
	errBIQueryEmpty = errcode.New(SegAnalytics, 1014, "维度与指标至少一项")
	errBITooMany    = errcode.New(SegAnalytics, 1015, "维度/指标/过滤超引擎上限")
	errBIOrderBad   = errcode.New(SegAnalytics, 1016, "排序字段不在查询列内")
	errEtlDateBad   = errcode.New(SegAnalytics, 1017, "ETL 日期不合法（yyyy-MM-dd）")
)

func registerSeg() bool {
	errcode.Register(SegAnalytics, "analytics")
	return true
}

// mustTenant 业务面租户（BFF authz 注入；事件消费侧由信封恢复）。
func mustTenant(ctx context.Context) (int64, error) {
	tid, err := tenantx.MustTenantFromCtx(ctx)
	if err != nil {
		return 0, errcode.ErrPermissionDenied.WithMsg("缺少租户上下文").WithCause(err)
	}
	return tid, nil
}
