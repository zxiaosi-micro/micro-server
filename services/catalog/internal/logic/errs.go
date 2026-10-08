// catalog 业务错误码（SegCatalog 段内偏移 1000+）。

package logic

import "github.com/zxiaosi-micro/micro-common/errcode"

var (
	errProductNotFound = errcode.New(errcode.SegCatalog, 1001, "商品不存在")
	errSkuNotFound     = errcode.New(errcode.SegCatalog, 1002, "SKU 不存在")
	errSkuCodeUsed     = errcode.New(errcode.SegCatalog, 1003, "SKU 编码已存在")
	errSkuTypeBad      = errcode.New(errcode.SegCatalog, 1004, "SKU 类型不合法")
	errPriceTypeBad    = errcode.New(errcode.SegCatalog, 1005, "价格类型不合法")
	errAmountBad       = errcode.New(errcode.SegCatalog, 1006, "价格格式不合法")
	errStartRuleBad    = errcode.New(errcode.SegCatalog, 1007, "起算规则不合法")
	errStationNotFound = errcode.New(errcode.SegCatalog, 1008, "场站模板不存在")
	errBomEmpty        = errcode.New(errcode.SegCatalog, 1009, "BOM 明细不能为空")
	errBomSkuNotFound  = errcode.New(errcode.SegCatalog, 1010, "BOM 明细含无效 SKU")
	errPolicyPeriodBad = errcode.New(errcode.SegCatalog, 1011, "质保期必须大于 0")
)
