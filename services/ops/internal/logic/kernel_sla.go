// SLA 内核：计时暂停/恢复（留痕顺延）+ ScanBreaches 分级升级
// （FR-OPS-006；策略快照在 contract 侧冻结，FR-CTR-007；扫描驱动 ADR-09）。
package logic

import (
	"context"
	"fmt"
	"time"

	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/internal/svc"

	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// tenantCtx 按租户重建 ctx（cron/扫描跨租户调下游前的 mustTenant 防御，known-pit 同款）。
func tenantCtx(tenantId int64) context.Context {
	return tenantx.WithTenant(context.Background(), tenantId)
}

// pauseSlaTimer 暂停（paused_at 写入；恢复时按 paused_at 计算顺延分钟）。
func pauseSlaTimer(ctx context.Context, sc *svc.ServiceContext, tid, ticketId int64, reason string) error {
	if reason == "" {
		return errSlaPausedNeed
	}
	timer, err := sc.Models.SlaTimer.FindOneByTicket(ctx, tid, ticketId)
	if err != nil {
		return errSlaNotFound
	}
	if timer.PausedAt.Valid {
		return errSlaPaused
	}
	return sc.Models.SlaTimer.Pause(ctx, tid, ticketId, time.Now(), reason)
}

// resumeSlaTimer 恢复：累计暂停分钟 + due_at 顺延（暂停期间不计时）。
func resumeSlaTimer(ctx context.Context, sc *svc.ServiceContext, tid, ticketId int64) error {
	timer, err := sc.Models.SlaTimer.FindOneByTicket(ctx, tid, ticketId)
	if err != nil {
		return errSlaNotFound
	}
	if !timer.PausedAt.Valid {
		return errSlaNotPaused
	}
	addMin := int64(time.Since(timer.PausedAt.Time).Minutes()) + 1 // 起止跨分钟进一
	newResp := timer.ResponseDueAt.Add(time.Duration(addMin) * time.Minute)
	newReso := timer.ResolveDueAt.Add(time.Duration(addMin) * time.Minute)
	return sc.Models.SlaTimer.Resume(ctx, tid, ticketId, addMin, newResp, newReso)
}

// scanSlaBreaches 定时扫描（跨租户平台任务，SQL 无租户条件；ADR-09）：
// 到期未达成 → 标记超时 + escalate_level+1 + 工单日志留痕 + 受理人通知。
func scanSlaBreaches(ctx context.Context, sc *svc.ServiceContext) error {
	timers, err := sc.Models.SlaTimer.ScanDueBreachesAll(ctx, 200)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, t := range timers {
		tctx := tenantCtx(t.TenantId)
		if !t.RespondedAt.Valid && !t.ResponseBreachAt.Valid && t.ResponseDueAt.Before(now) {
			if ok, err := sc.Models.SlaTimer.MarkBreach(tctx, t.TenantId, t.TimerId, "response_breach_at", now); err == nil && ok {
				logSlaBreach(sc, t, "响应超时")
				notifySlaBreach(sc, t, "响应超时")
			}
		}
		if !t.ResolvedAt.Valid && !t.ResolveBreachAt.Valid && t.ResolveDueAt.Before(now) {
			if ok, err := sc.Models.SlaTimer.MarkBreach(tctx, t.TenantId, t.TimerId, "resolve_breach_at", now); err == nil && ok {
				logSlaBreach(sc, t, "解决超时")
				notifySlaBreach(sc, t, "解决超时")
			}
		}
	}
	return nil
}

func logSlaBreach(sc *svc.ServiceContext, t *model.SlaTimer, what string) {
	// 工单日志留痕（SLA 升级可见于工单时间线）
	_ = sc.Models.TicketLog.InsertTx(tenantCtx(t.TenantId), sc.Conn, &model.TicketLog{
		LogId: sc.Snowflake.MustNextID(), TicketId: t.TicketId,
		Action: "sla_escalate", FromStatus: "", ToStatus: "",
		OperatorUid: 0, Note: fmt.Sprintf("SLA %s（第 %d 次升级）", what, t.EscalateLevel+1),
		TenantId: t.TenantId,
	})
}

func notifySlaBreach(sc *svc.ServiceContext, t *model.SlaTimer, what string) {
	// 派单受理人通知（0=未派单不发）
	tk, err := sc.Models.Ticket.FindOneScoped(tenantCtx(t.TenantId), t.TenantId, t.TicketId)
	if err != nil {
		return
	}
	notify(sc, t.TenantId, tk.AssigneeUid.Int64, "sla_escalated", "SLA",
		fmt.Sprintf("sla:%d", t.TicketId), map[string]string{
			"ticket_no": tk.TicketNo, "what": what, "level": fmt.Sprintf("%d", t.EscalateLevel+1),
		})
}
