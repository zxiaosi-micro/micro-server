package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRebateSettlementsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 返利结算列表
func NewListRebateSettlementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRebateSettlementsLogic {
	return &ListRebateSettlementsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListRebateSettlementsLogic) ListRebateSettlements(req *types.RebateListReq) (resp *types.RebateListResp, err error) {
	out, err := l.svcCtx.Finance.ListRebateSettlement(l.ctx, &finpb.ListRebateSettlementReq{
		Period: req.Period, Status: req.Status, PartyId: parseI64(req.PartyId),
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.RebateListResp{Total: out.Total, List: []types.RebateSettlementItem{}}
	for _, r := range out.List {
		resp.List = append(resp.List, types.RebateSettlementItem{
			SettlementId: i64s(r.SettlementId), Period: r.Period, PartyId: i64s(r.PartyId),
			Amount: r.Amount, Status: r.Status, RuleSnapshot: r.RuleSnapshot,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}
	return resp, nil
}
