// dw.go · 数仓 ETL 内核（S10-02）：单日 ETL → 批次台账；缺失批次补跑（调度对账）。
//
// 日调度：cronx 每小时检查「昨日批次」缺失即补跑（服务重启后自愈；
// K8s CronJob 候选形态见 ADR-09 阶段二——届时 CronJob 调 RunDwEtl RPC，本 cron 退化为兜底）。
// 手动触发：RunDwEtl RPC（补历史区间）；重跑幂等（ODKU/IGNORE 同值 affected=0）。

package logic

import (
	"context"
	"time"

	"micro-server/services/analytics/internal/model"
	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

// RunDwEtlCore ETL 内核：[start,end] 逐日执行（end 空=单日）；返回逐日计量。
func RunDwEtlCore(ctx context.Context, sc *svc.ServiceContext, start, end time.Time) ([]*model.DwEtlStat, []time.Time, error) {
	if end.Before(start) {
		return nil, nil, errEtlDateBad.WithMsg("date_end 早于 date_start")
	}
	var stats []*model.DwEtlStat
	var days []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		stat, err := sc.Models.Dw.EtlDay(ctx, d)
		if err != nil {
			return nil, nil, err
		}
		stats = append(stats, stat)
		days = append(days, d)
	}
	return stats, days, nil
}

// RunDwEtlMissing 补齐缺失批次（cron 调度口：昨日无台账即补）。
func RunDwEtlMissing(ctx context.Context, sc *svc.ServiceContext) error {
	yesterday := StatDateOf(time.Now().AddDate(0, 0, -1))
	missing, err := sc.Models.Dw.MissingDays(ctx, yesterday, yesterday)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	for _, day := range missing {
		start := time.Now()
		stat, err := sc.Models.Dw.EtlDay(ctx, day)
		if err != nil {
			return err
		}
		if err := sc.Models.Dw.InsertEtlLog(ctx, day, stat, time.Since(start).Milliseconds()); err != nil {
			return err
		}
		logx.Infof("dw etl done date=%s order=%d payment=%d", day.Format("2006-01-02"), stat.FactOrderRows, stat.FactPayRows)
	}
	return nil
}

// RunDwEtl RPC 入口（手动触发/补历史）。
func (l *RunDwEtlLogic) RunDwEtl(in *pb.RunDwEtlReq) (*pb.RunDwEtlResp, error) {
	if _, err := mustTenant(l.ctx); err != nil {
		return nil, err
	}
	start, err := parseDay(in.DateStart)
	if err != nil || in.DateStart == "" {
		return nil, errEtlDateBad
	}
	end := start
	if in.DateEnd != "" {
		if end, err = parseDay(in.DateEnd); err != nil {
			return nil, errEtlDateBad
		}
	}
	stats, days, err := RunDwEtlCore(l.ctx, l.svcCtx, start, end)
	if err != nil {
		return nil, err
	}
	resp := &pb.RunDwEtlResp{Days: []*pb.DwEtlDayResult{}}
	for i, stat := range stats {
		resp.Days = append(resp.Days, &pb.DwEtlDayResult{
			EtlDate:        days[i].Format("2006-01-02"),
			DimDateRows:    stat.DimDateRows,
			DimTenantRows:  stat.DimTenantRows,
			DimPartyRows:   stat.DimPartyRows,
			FactOrderRows:  stat.FactOrderRows,
			FactPaymentRows: stat.FactPayRows,
		})
		// 手动触发也留台账（重跑覆盖计量）
		if err := l.svcCtx.Models.Dw.InsertEtlLog(l.ctx, days[i], stat, 0); err != nil {
			return nil, err
		}
	}
	return resp, nil
}
