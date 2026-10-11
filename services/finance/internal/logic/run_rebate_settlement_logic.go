package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunRebateSettlementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRunRebateSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunRebateSettlementLogic {
	return &RunRebateSettlementLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 经销商返利（S10-03：规则快照可追溯；月度结算/确认/对账，规则源 party.dealer_ext.rebate_rule）----
func (l *RunRebateSettlementLogic) RunRebateSettlement(in *pb.RunRebateSettlementReq) (*pb.RunRebateSettlementResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	start, end, err := parseRebatePeriod(in.Period)
	if err != nil {
		return nil, err
	}
	partyIds := []int64{in.PartyId}
	if in.PartyId <= 0 {
		if partyIds, err = listDealers(l.ctx, l.svcCtx); err != nil {
			return nil, err
		}
	}
	resp := &pb.RunRebateSettlementResp{Results: []*pb.RebateSettlementResult{}}
	for _, pid := range partyIds {
		r, err := settleDealer(l.ctx, l.svcCtx, tid, pid, in.Period, start, end)
		if err != nil {
			return nil, err
		}
		if r != nil {
			resp.Results = append(resp.Results, r)
		}
	}
	logx.WithContext(l.ctx).Infof("rebate settlement period=%s dealers=%d settled=%d", in.Period, len(partyIds), len(resp.Results))
	return resp, nil
}
