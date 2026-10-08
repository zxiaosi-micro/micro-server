// order 业务错误码（order 段 50000：交易域按 10000 步长续接）。

package logic

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// SegSegOrder 段 = micro-common 预注册段（S5 交易域，勿再 Register）。
const SegOrder = errcode.SegOrder

var (
	errOrderNotFound   = errcode.New(SegOrder, 1001, "订单不存在")
	errOrderNoUsed     = errcode.New(SegOrder, 1002, "订单号已存在(重复请求)")
	errOrderStatus     = errcode.New(SegOrder, 1003, "订单状态不允许该操作")
	errOrderTypeBad    = errcode.New(SegOrder, 1004, "订单类型不合法")
	errItemsRequired   = errcode.New(SegOrder, 1005, "订单明细不能为空")
	errQtyBad          = errcode.New(SegOrder, 1006, "数量必须大于 0")
	errAmountBad       = errcode.New(SegOrder, 1007, "金额快照解析失败")
	errChannelBad      = errcode.New(SegOrder, 1008, "支付渠道不合法")
	errReturnNotFound  = errcode.New(SegOrder, 1009, "退货单不存在")
	errReturnStatus    = errcode.New(SegOrder, 1010, "退货单状态不允许该操作")
	errShipmentBad     = errcode.New(SegOrder, 1011, "发货单不存在")
	errShipmentStatus  = errcode.New(SegOrder, 1012, "发货单状态不允许该操作")
	errSagaNotFound    = errcode.New(SegOrder, 1013, "Saga 不存在")
	errSagaNotRetryable = errcode.New(SegOrder, 1014, "Saga 已终结(不可重推)")
	errItemNotFound    = errcode.New(SegOrder, 1015, "退货明细与原单不符")
	errDownstreamMiss  = errcode.New(SegOrder, 1016, "下游服务未配置")
)

// 下游业务错误码（跨服务判定用，与各服务 errs.go 对齐）。
const (
	codeInvInsufficient = 41004 // inventory: 库存不足
	codeInvTxnReplay    = 41005 // inventory: 重复请求(已处理，幂等成功)
	codeInvLockedShort  = 41006 // inventory: 锁定余量不足
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

// bizCodeOf 提取下游业务码：OutgoingInterceptor 已把 gRPC status 还原为 CodeError，
// 这里按 CodeError 链解析（本地构造的 CodeError 同样命中），02 §8 无损往返的客户端侧。
func bizCodeOf(err error) (int64, bool) {
	if ce, ok := errcode.FromError(err); ok {
		return ce.Code(), true
	}
	return 0, false
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

