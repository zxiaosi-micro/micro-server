// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package order

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PayOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPayOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayOrderLogic {
	return &PayOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 发起支付(Saga 步骤3, perm: trade:order:pay)
func (l *PayOrderLogic) PayOrder(req *types.OrderPayReq) (resp *types.OrderPayResp, err error) {
	respOut, err := l.svcCtx.Order.PayOrder(l.ctx, &opb.PayOrderReq{OrderNo: req.OrderNo, Channel: req.Channel})
	if err != nil {
		return nil, err
	}
	return &types.OrderPayResp{PaymentNo: respOut.PaymentNo, Status: respOut.Status, PayParams: respOut.PayParams}, nil
}
