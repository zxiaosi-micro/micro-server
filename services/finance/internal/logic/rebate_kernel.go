// rebate_kernel.go · 经销商返利共享内核（S10-03：规则快照可追溯）。
//
// 规则源 party.dealer_ext.rebate_rule JSON：{"rate":0.03,"settle":"MONTHLY"}；
// 结算口径：period 内 payer=经销商 且状态 PAID/SETTLED 的支付单 amount 合计 × rate
// （分域整数运算四舍五入到分）；QUARTER 规则仅季末月（03/06/09/12）出单。
// 幂等：uk(tenant,period,party) Upsert——金额仅 PENDING 可覆盖（CONFIRMED/PAID 业务锁），
// 重跑同值 affected=0；对账 = 存储金额 vs 当前基数×快照费率重算。

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"
	partypb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

// rebateRule 返利规则（dealer_ext.rebate_rule 快照源）。
type rebateRule struct {
	Rate   float64 `json:"rate"`   // 返利率 0~1（0.03 = 3%）
	Settle string  `json:"settle"` // MONTHLY/QUARTER
}

// parseRebatePeriod "yyyy-MM" → 月首/次月首（本地时区）。
func parseRebatePeriod(period string) (time.Time, time.Time, error) {
	if len(period) != 7 || period[4] != '-' {
		return time.Time{}, time.Time{}, errRebatePeriodBad
	}
	start, err := time.ParseInLocation("2006-01", period, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errRebatePeriodBad
	}
	return start, start.AddDate(0, 1, 0), nil
}

// parseRebateRule 规则 JSON 解析（rate>0 且 settle 合法；QUARTER 非季末月返回 nil 跳过）。
func parseRebateRule(raw string, month int) (*rebateRule, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil // 未配规则：跳过
	}
	var r rebateRule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, errRebateRuleBad
	}
	if r.Rate <= 0 || r.Rate > 1 {
		return nil, errRebateRuleBad
	}
	switch r.Settle {
	case "", "MONTHLY":
		r.Settle = "MONTHLY"
	case "QUARTER":
		if month != 3 && month != 6 && month != 9 && month != 12 {
			return nil, nil
		}
	default:
		return nil, errRebateRuleBad
	}
	return &r, nil
}

// rebateCalcCents 计提金额（分域整数 × 费率，四舍五入到分）。
func rebateCalcCents(baseCents int64, rate float64) int64 {
	return int64(math.Round(float64(baseCents) * rate))
}

// settleDealer 单经销商结算（返回 nil = 无规则/非出单月跳过）；ODKU 幂等（PENDING 才可覆盖）。
func settleDealer(ctx context.Context, sc *svc.ServiceContext, tid, partyId int64, period string,
	start, end time.Time) (*pb.RebateSettlementResult, error) {

	ext, err := sc.Party.GetDealerExt(ctx, &partypb.GetDealerExtReq{PartyId: partyId})
	if err != nil {
		return nil, err
	}
	rawRule := ""
	if ext.DealerExt != nil {
		rawRule = ext.DealerExt.RebateRule
	}
	rule, err := parseRebateRule(rawRule, int(start.Month()))
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, nil
	}
	baseAmount, payCnt, err := sc.Models.Rebate.SumPaidAmount(ctx, tid, partyId,
		start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	baseCents, err := parseCents(baseAmount)
	if err != nil {
		return nil, err
	}
	amountCents := rebateCalcCents(baseCents, rule.Rate)

	// 规则快照（结算金额可追溯，验收项）：原始规则 + 计提基数 + 费率 + 算式 + 单数
	snapshot, err := json.Marshal(map[string]any{
		"rule":          rule,
		"rule_raw":      rawRule,
		"base_amount":   baseAmount,
		"rate":          rule.Rate,
		"calc":          fmt.Sprintf("%s×%g（四舍五入到分）", baseAmount, rule.Rate),
		"payment_count": payCnt,
		"settled_at":    time.Now().UnixMilli(),
	})
	if err != nil {
		return nil, err
	}
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return sc.Models.Rebate.UpsertTx(ctx, session, &model.RebateSettlement{
			SettlementId: sc.Snowflake.MustNextID(),
			Period:       period,
			PartyId:      partyId,
			RuleSnapshot: sqlString(string(snapshot)),
			Amount:       float64(amountCents) / 100,
			Status:       "PENDING",
			TenantId:     tid,
			UpdatedBy:    sqlInt64(ctxkit.UID(ctx)),
		})
	})
	if err != nil {
		return nil, err
	}
	return &pb.RebateSettlementResult{
		PartyId:      partyId,
		Amount:       centsToAmount(amountCents),
		BaseAmount:   baseAmount,
		Rate:         strconv.FormatFloat(rule.Rate, 'f', -1, 64),
		PaymentCount: payCnt,
	}, nil
}

// listDealers 经销商清单（party.ListParty(type=DEALER) 分页遍历；规则源就绪校验）。
func listDealers(ctx context.Context, sc *svc.ServiceContext) ([]int64, error) {
	if sc.Party == nil {
		return nil, errRebatePartyNotReady
	}
	var out []int64
	page := int64(1)
	for {
		resp, err := sc.Party.ListParty(ctx, &partypb.ListPartyReq{
			Type: "DEALER", Page: page, Size: 100,
		})
		if err != nil {
			return nil, err
		}
		for _, p := range resp.List {
			out = append(out, p.PartyId)
		}
		if len(resp.List) < 100 || int64(len(out)) >= resp.Total {
			break
		}
		page++
	}
	return out, nil
}

// rebateItem 行 → proto 视图（金额 DECIMAL string 防精度丢失）。
func rebateItem(r *model.RebateSettlement) *pb.RebateSettlementItem {
	return &pb.RebateSettlementItem{
		SettlementId: r.SettlementId,
		Period:       r.Period,
		PartyId:      r.PartyId,
		Amount:       centsToAmount(int64(math.Round(r.Amount * 100))),
		Status:       r.Status,
		RuleSnapshot: nullStr(r.RuleSnapshot),
		CreatedAt:    r.CreatedAt.UnixMilli(),
		UpdatedAt:    r.UpdatedAt.UnixMilli(),
	}
}

// _ logx 引用保持（kernel 内日志由调用方打）。
var _ = logx.Info
