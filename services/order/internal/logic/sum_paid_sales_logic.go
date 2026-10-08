package logic

import (
	"context"
	"time"

	"micro-server/services/order/internal/svc"
	"micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SumPaidSalesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSumPaidSalesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SumPaidSalesLogic {
	return &SumPaidSalesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SumPaidSales 已支付销售汇总（paid_at ∈ [from,to)；PAID 起含）。
func (l *SumPaidSalesLogic) SumPaidSales(in *pb.SumPaidSalesReq) (*pb.SumPaidSalesResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	to := time.UnixMilli(in.To)
	if in.To <= 0 {
		to = time.Now()
	}
	from := time.UnixMilli(in.From)
	if in.From <= 0 {
		from = to.AddDate(0, 0, -1)
	}
	total, count, err := l.svcCtx.Models.Order.SumPaidSales(l.ctx, tid, from, to)
	if err != nil {
		return nil, err
	}
	return &pb.SumPaidSalesResp{
		TotalAmount: centsToAmount(floatToCents(total)),
		OrderCount:  count,
	}, nil
}
