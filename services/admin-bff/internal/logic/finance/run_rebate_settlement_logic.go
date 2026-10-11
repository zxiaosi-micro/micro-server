package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunRebateSettlementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 返利月度结算(规则快照可追溯)
func NewRunRebateSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunRebateSettlementLogic {
	return &RunRebateSettlementLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RunRebateSettlementLogic) RunRebateSettlement(req *types.RebateRunReq) (resp *types.RebateRunResp, err error) {
	out, err := l.svcCtx.Finance.RunRebateSettlement(l.ctx, &finpb.RunRebateSettlementReq{
		Period: req.Period, PartyId: parseI64(req.PartyId),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.RebateRunResp{Results: []types.RebateRunResult{}}
	for _, r := range out.Results {
		resp.Results = append(resp.Results, types.RebateRunResult{
			PartyId: i64s(r.PartyId), Amount: r.Amount, BaseAmount: r.BaseAmount,
			Rate: r.Rate, PaymentCount: r.PaymentCount,
		})
	}
	return resp, nil
}
