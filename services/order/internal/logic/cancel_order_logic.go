package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

type CancelOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelOrderLogic {
	return &CancelOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CancelOrder 取消订单（用户）：步骤≤3 可取消，补偿=释放已锁库存（Saga 补偿纪律，02 §9.1）。
func (l *CancelOrderLogic) CancelOrder(in *pb.CancelOrderReq) (*pb.CancelOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := cancelOrderInternal(l.ctx, l.svcCtx, tid, in.OrderNo, in.Reason, "USER", "CANCELLED"); err != nil {
		return nil, err
	}
	return &pb.CancelOrderResp{}, nil
}

// cancelOrderInternal 取消内核（用户取消与支付超时扫描共用）。
// finalStatus: CANCELLED（用户） / PAY_TIMEOUT（超时）；事件：order_cancelled / order_pay_timeout。
func cancelOrderInternal(ctx context.Context, sc *svc.ServiceContext, tid int64,
	orderNo, reason, cancelledBy, finalStatus string) error {

	// 事务1：行锁校验 + 标记 COMPENSATING（阻止并发支付/推进）
	var orderId int64
	var lockKeys []string
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		order, err := sc.Models.Order.FindOneByNoForUpdateTx(ctx, session, tid, orderNo)
		if err != nil {
			return err
		}
		if order.Status != "CREATED" && order.Status != "LOCKED" && order.Status != "PAYING" {
			return errOrderStatus.WithMsg("订单已出库或终结，请走退货流程: " + order.Status)
		}
		saga, err := sc.Models.Saga.FindOneForUpdateTx(ctx, session, tid, orderNo)
		if err != nil {
			return err
		}
		if saga.Status == "DONE" || saga.Status == "CANCELLED" {
			return errOrderStatus.WithMsg("Saga 已终结: " + saga.Status)
		}
		orderId = order.OrderId
		lockKeys = parseSagaContext(saga.ContextJson).LockKeys
		// 置 COMPENSATING（补偿也是业务动作，需要防并发重入）
		return sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "COMPENSATING", reason)
	})
	_ = orderId
	if err != nil {
		if err == model.ErrNotFound {
			return errOrderNotFound
		}
		return err
	}

	// 补偿：按明细释放已锁库存（幂等；失败留痕走对账兜底，E10）
	if len(lockKeys) > 0 {
		items, ierr := sc.Models.OrderItem.FindByOrder(ctx, tid, orderId)
		if ierr == nil {
			releaseLockedItems(ctx, sc, tid, orderNo, items, nil, "取消: "+reason)
		}
	}

	// 事务2：终态 + 事件（order_cancelled / order_pay_timeout）
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		saga, err := sc.Models.Saga.FindOneByOrderNo(ctx, tid, orderNo)
		if err != nil {
			return err
		}
		if err := sc.Models.Saga.UpdateStatusTx(ctx, session, tid, saga.SagaId, "CANCELLED", reason); err != nil {
			return err
		}
		if err := sc.Models.Order.CASStatusTx(ctx, session, tid, orderId,
			[]string{"CREATED", "LOCKED", "PAYING"}, finalStatus); err != nil {
			return err
		}
		if finalStatus == "PAY_TIMEOUT" {
			var exp int64
			if o, err := sc.Models.Order.FindOneScoped(ctx, tid, orderId); err == nil && o.PayExpireAt.Valid {
				exp = o.PayExpireAt.Time.UnixMilli()
			}
			return eventbus.Emit(ctx, session, orderPayTimeoutEvent(tid, orderNo, exp))
		}
		return eventbus.Emit(ctx, session, orderCancelledEvent(tid, orderNo, reason, cancelledBy))
	})
	if err != nil {
		return err
	}

	logx.WithContext(ctx).Errorf("订单已取消 order_no=%s by=%s reason=%s", orderNo, cancelledBy, reason)
	return nil
}
