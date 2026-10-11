package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReconcileRebateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 返利月度对账(差异为 0 验收)
func NewReconcileRebateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReconcileRebateLogic {
	return &ReconcileRebateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ReconcileRebateLogic) ReconcileRebate(req *types.RebateReconcileReq) (resp *types.RebateReconcileResp, err error) {
	out, err := l.svcCtx.Finance.ReconcileRebate(l.ctx, &finpb.ReconcileRebateReq{Period: req.Period})
	if err != nil {
		return nil, err
	}
	resp = &types.RebateReconcileResp{DiffCount: out.DiffCount, Diffs: []types.RebateDiffItem{}}
	for _, d := range out.Diffs {
		resp.Diffs = append(resp.Diffs, types.RebateDiffItem{
			SettlementId: i64s(d.SettlementId), PartyId: i64s(d.PartyId),
			StoredAmount: d.StoredAmount, RecomputedAmount: d.RecomputedAmount,
		})
	}
	return resp, nil
}
