package logic

import (
	"context"

	"micro-server/services/order/internal/model"
	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PayOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPayOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayOrderLogic {
	return &PayOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PayOrder Saga 步骤3：发起支付（用户触达 finance.CreatePayment；幂等——重复调用返回既有支付单）。
func (l *PayOrderLogic) PayOrder(in *pb.PayOrderReq) (*pb.PayOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Channel != "WECHAT" && in.Channel != "ALIPAY" && in.Channel != "BANK_OFFLINE" {
		return nil, errChannelBad
	}
	sc := l.svcCtx

	// 事务1：锁定订单 + Saga，校验可支付
	var order *model.Order
	var saga *model.Saga
	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var e error
		order, e = sc.Models.Order.FindOneByNoForUpdateTx(ctx, session, tid, in.OrderNo)
		if e != nil {
			return e
		}
		saga, e = sc.Models.Saga.FindOneForUpdateTx(ctx, session, tid, in.OrderNo)
		if e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errOrderNotFound
		}
		return nil, err
	}

	// 幂等：已在支付中 → 返回既有支付单（不重复向渠道发起）
	scx := parseSagaContext(saga.ContextJson)
	if order.Status == "PAYING" && scx.PaymentNo != "" {
		return &pb.PayOrderResp{PaymentNo: scx.PaymentNo, Status: "PAYING"}, nil
	}
	if order.Status != "LOCKED" || saga.CurrentStep < stepPay {
		return nil, errOrderStatus.WithMsg("订单当前状态不可支付: " + order.Status)
	}
	if sc.Finance == nil {
		return nil, errDownMiss()
	}

	// RPC：finance.CreatePayment（事务外；Saga 步骤3 幂等键 saga_id+pay 由 finance 侧 order_no 活跃单兜底）
	amountCents := floatToCents(order.TotalAmount)
	var payRes *finpb.CreatePaymentResp
	err = rpcCall(l.ctx, func(ctx context.Context) error {
		var e error
		payRes, e = sc.Finance.CreatePayment(ctx, &finpb.CreatePaymentReq{
			OrderNo: in.OrderNo, Amount: centsToAmount(amountCents), Channel: in.Channel,
			PayerPartyId: order.BuyerPartyId.Int64,
		})
		return e
	})
	if err != nil {
		return nil, err // finance 业务码经 gRPC 无损透传（渠道降级/参数错误）
	}

	// 事务2：登记支付单号 + 订单进入 PAYING
	err = sc.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		scx2 := parseSagaContext(saga.ContextJson)
		scx2.PaymentNo = payRes.PaymentNo
		scx2.PayChannel = in.Channel
		if err := sc.Models.Saga.UpdateContextTx(ctx, session, tid, saga.SagaId, mustJSON(scx2)); err != nil {
			return err
		}
		return sc.Models.Order.CASStatusTx(ctx, session, tid, order.OrderId,
			[]string{"LOCKED"}, "PAYING")
	})
	if err != nil && err != model.ErrStatusConflict {
		return nil, err
	}

	l.Infof("支付已发起 order_no=%s payment_no=%s channel=%s", in.OrderNo, payRes.PaymentNo, in.Channel)
	return &pb.PayOrderResp{
		PaymentNo: payRes.PaymentNo,
		Status:    payRes.Status,
		PayParams: payRes.PayParams,
	}, nil
}
