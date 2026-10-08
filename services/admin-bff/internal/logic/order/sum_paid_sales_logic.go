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

type SumPaidSalesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSumPaidSalesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SumPaidSalesLogic {
	return &SumPaidSalesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SumPaidSalesLogic) SumPaidSales(req *types.SalesSumReq) (resp *types.SalesSumResp, err error) {
	respOut, err := l.svcCtx.Order.SumPaidSales(l.ctx, &opb.SumPaidSalesReq{From: req.From, To: req.To})
	if err != nil {
		return nil, err
	}
	return &types.SalesSumResp{TotalAmount: respOut.TotalAmount, OrderCount: int(respOut.OrderCount)}, nil
}
