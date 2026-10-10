// inventory 业务错误码（SegInventory 段内偏移 1000+；ErrTxnReplay 幂等重放见 02 §9.2）。

package logic

import "github.com/zxiaosi-micro/micro-common/errcode"

var (
	errWarehouseNotFound = errcode.New(errcode.SegInventory, 1001, "仓库不存在")
	errWarehouseCodeUsed = errcode.New(errcode.SegInventory, 1002, "仓库编码已存在")
	errInventoryNotFound = errcode.New(errcode.SegInventory, 1003, "库存未建档")
	// ErrInsufficientStock 库存不足（DB 余量守卫 affected==0 / Redis 预扣负值回滚）。
	ErrInsufficientStock = errcode.New(errcode.SegInventory, 1004, "库存不足")
	// ErrTxnReplay 幂等重放：(biz_type,biz_no) 唯一约束 1062 转业务语义（02 §9.2 第 3 步）。
	ErrTxnReplay          = errcode.New(errcode.SegInventory, 1005, "重复请求(该业务单号已处理)")
	ErrLockedInsufficient = errcode.New(errcode.SegInventory, 1006, "锁定库存不足")
	errStocktakeNotFound  = errcode.New(errcode.SegInventory, 1007, "盘点单不存在")
	errStocktakeStatus    = errcode.New(errcode.SegInventory, 1008, "盘点单状态不允许该操作")
	errWorkOrderRequired  = errcode.New(errcode.SegInventory, 1009, "备件领用必须关联工单号")
	errQtyBad             = errcode.New(errcode.SegInventory, 1010, "数量必须大于 0")
	errBizNoBad           = errcode.New(errcode.SegInventory, 1011, "业务单号不能为空")
	errSnQtyMismatch      = errcode.New(errcode.SegInventory, 1017, "SN 明细数量与入库数量不一致")
	errBizTypeBad         = errcode.New(errcode.SegInventory, 1012, "业务类型不合法")
	errCountedBad         = errcode.New(errcode.SegInventory, 1013, "实盘数不能为负")
)
