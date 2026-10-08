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

type ApproveReturnOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveReturnOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveReturnOrderLogic {
	return &ApproveReturnOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ApproveReturnOrder 退货审批：通过 → order_return_approved 事件（finance 消费原路退回）；
// 驳回 → REJECTED。退款回写由 payment_refunded 事件驱动（→ REFUNDED）。
func (l *ApproveReturnOrderLogic) ApproveReturnOrder(in *pb.ApproveReturnOrderReq) (*pb.ApproveReturnOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	sc := l.svcCtx

	// 行锁取退货单（审批并发入口）
	ro, err := sc.Models.ReturnOrder.FindOneByNo(l.ctx, tid, in.ReturnNo)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errReturnNotFound
		}
		return nil, err
	}
	if ro.Status != "APPLYING" {
		return nil, errReturnStatus.WithMsg("当前状态: " + ro.Status)
	}

	if !in.Approve {
		err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.ReturnOrder.MarkRejectedTx(ctx, session, tid, ro.ReturnId, in.Remark)
		})
		if err != nil {
			return nil, err
		}
		l.Infof("退货驳回 return_no=%s remark=%s", in.ReturnNo, in.Remark)
		return &pb.ApproveReturnOrderResp{}, nil
	}

	// 退款金额：退货明细 × 原单单价快照
	refundCents, err := l.refundCents(tid, ro)
	if err != nil {
		return nil, err
	}

	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 事务内行锁重校验（防并发双审批）
		cur, err := sc.Models.ReturnOrder.FindOneByNoForUpdateTx(ctx, session, tid, in.ReturnNo)
		if err != nil {
			return err
		}
		if cur.Status != "APPLYING" {
			return errReturnStatus
		}
		if err := sc.Models.ReturnOrder.MarkApprovedTx(ctx, session, tid, ro.ReturnId, float64(refundCents)/100); err != nil {
			return err
		}
		// order_return_approved（finance 消费 → CreateRefund 原路退回，FR-FIN-002）
		return eventbus.Emit(ctx, session, orderReturnApprovedEvent(tid, ro.ReturnNo, ro.OrderNo,
			centsToAmount(refundCents), in.Remark))
	})
	if err != nil {
		if err == model.ErrStatusConflict || err == errReturnStatus {
			return nil, errReturnStatus
		}
		return nil, err
	}

	l.Infof("退货已批准 return_no=%s order_no=%s refund=%s（等待退款回写）",
		in.ReturnNo, ro.OrderNo, centsToAmount(refundCents))
	return &pb.ApproveReturnOrderResp{}, nil
}

// refundCents 退货明细 × 原单单价快照（金额走分）。
func (l *ApproveReturnOrderLogic) refundCents(tid int64, ro *model.ReturnOrder) (int64, error) {
	items, err := l.svcCtx.Models.ReturnItem.FindByReturn(l.ctx, tid, ro.ReturnId)
	if err != nil {
		return 0, err
	}
	orderItems, err := l.svcCtx.Models.OrderItem.FindByOrder(l.ctx, tid, ro.OrderId)
	if err != nil {
		return 0, err
	}
	priceBySku := map[int64]int64{}
	for _, oi := range orderItems {
		priceBySku[oi.SkuId] = floatToCents(oi.UnitPrice)
	}
	var total int64
	for _, it := range items {
		price, ok := priceBySku[it.SkuId]
		if !ok {
			return 0, errItemNotFound
		}
		total += price * it.Qty
	}
	return total, nil
}
