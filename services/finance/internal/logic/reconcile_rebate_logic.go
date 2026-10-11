package logic

import (
	"context"
	"encoding/json"
	"math"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReconcileRebateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReconcileRebateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReconcileRebateLogic {
	return &ReconcileRebateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ReconcileRebate 月度对账（验收：差异为 0）：
// 存储金额 vs 按（快照基数 × 快照费率）重算（快照自洽追溯）
// + 当前基数 vs 快照基数（源支付单是否变化，如退款）。任一不一致计入差异清单。
func (l *ReconcileRebateLogic) ReconcileRebate(in *pb.ReconcileRebateReq) (*pb.ReconcileRebateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	start, end, err := parseRebatePeriod(in.Period)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Rebate.ListByPeriod(l.ctx, tid, in.Period)
	if err != nil {
		return nil, err
	}
	resp := &pb.ReconcileRebateResp{Diffs: []*pb.RebateDiffItem{}}
	for _, r := range list {
		var snap struct {
			BaseAmount string  `json:"base_amount"`
			Rate       float64 `json:"rate"`
		}
		if raw := nullStr(r.RuleSnapshot); raw != "" {
			if err := json.Unmarshal([]byte(raw), &snap); err != nil {
				resp.DiffCount++
				resp.Diffs = append(resp.Diffs, &pb.RebateDiffItem{
					SettlementId: r.SettlementId, PartyId: r.PartyId,
					StoredAmount: centsToAmount(int64(math.Round(r.Amount * 100))),
				})
				continue
			}
		}
		// 快照自洽：存储金额 == 快照基数×费率；源数据一致：当前基数 == 快照基数
		snapBaseCents, _ := parseCents(snap.BaseAmount)
		snapCalc := rebateCalcCents(snapBaseCents, snap.Rate)
		curAmount, _, err := l.svcCtx.Models.Rebate.SumPaidAmount(l.ctx, tid, r.PartyId,
			start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
		if err != nil {
			return nil, err
		}
		curCents, _ := parseCents(curAmount)
		storedCents := int64(math.Round(r.Amount * 100))
		if storedCents != snapCalc || curCents != snapBaseCents {
			resp.DiffCount++
			resp.Diffs = append(resp.Diffs, &pb.RebateDiffItem{
				SettlementId:     r.SettlementId,
				PartyId:          r.PartyId,
				StoredAmount:     centsToAmount(storedCents),
				RecomputedAmount: centsToAmount(rebateCalcCents(curCents, snap.Rate)),
			})
		}
	}
	logx.WithContext(l.ctx).Infof("rebate reconcile period=%s rows=%d diffs=%d", in.Period, len(list), resp.DiffCount)
	return resp, nil
}
