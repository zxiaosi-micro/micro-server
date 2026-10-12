// sla_timer custom 覆写：快照计时/暂停恢复/超时扫描（FR-OPS-006 + ADR-09）。
package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ SlaTimerModel = (*customSlaTimerModel)(nil)

type (
	// SlaTimerModel SLA 计时器（策略快照冻结在 contract 侧；本表管计时）。
	SlaTimerModel interface {
		slaTimerModel
		// InsertTx 事务内插入（uk_timer_ticket 冲突 = 重放吞掉）。
		InsertTx(ctx context.Context, session sqlx.Session, data *SlaTimer) error
		FindOneByTicket(ctx context.Context, tenantId, ticketId int64) (*SlaTimer, error)
		// Pause 暂停（计时中→暂停；留原因）。
		Pause(ctx context.Context, tenantId, ticketId int64, pausedAt time.Time, reason string) error
		// Resume 恢复（累计暂停分钟 + due_at 顺延）。
		Resume(ctx context.Context, tenantId, ticketId int64, addMinutes int64, newResponseDue, newResolveDue time.Time) error
		// MarkResponded 接单即响应达成。
		MarkResponded(ctx context.Context, tenantId, ticketId int64, at time.Time) error
		// MarkResolved 验收即解决达成。
		MarkResolved(ctx context.Context, tenantId, ticketId int64, at time.Time) error
		// MarkBreach 超时分级升级（escalate_level+1）。
		MarkBreach(ctx context.Context, tenantId, timerId int64, column string, at time.Time) (bool, error)
		// ScanDueBreaches 未达成的到期计时（单租户，ADR-09 扫描源）。
		ScanDueBreaches(ctx context.Context, tenantId int64, now time.Time, limit int) ([]*SlaTimer, error)
		// ScanDueBreachesAll 跨租户平台扫描（ScanBreaches cron 用，SQL 无租户条件）。
		ScanDueBreachesAll(ctx context.Context, limit int) ([]*SlaTimer, error)
	}

	customSlaTimerModel struct {
		*defaultSlaTimerModel
		conn sqlx.SqlConn
	}
)

// NewSlaTimerModel 构造。
func NewSlaTimerModel(conn sqlx.SqlConn) SlaTimerModel {
	return &customSlaTimerModel{defaultSlaTimerModel: newSlaTimerModel(conn), conn: conn}
}

func (m *customSlaTimerModel) InsertTx(ctx context.Context, session sqlx.Session, data *SlaTimer) error {
	query := "insert into `sla_timer` (`timer_id`,`ticket_id`,`strategy_id`,`strategy_snapshot`,`response_due_at`,`resolve_due_at`,`tenant_id`) values (?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.TimerId, data.TicketId, data.StrategyId, data.StrategySnapshot,
		data.ResponseDueAt, data.ResolveDueAt, data.TenantId)
	return err
}

func (m *customSlaTimerModel) FindOneByTicket(ctx context.Context, tenantId, ticketId int64) (*SlaTimer, error) {
	var res SlaTimer
	query := "select " + slaTimerRows + " from `sla_timer` where `ticket_id` = ? and `tenant_id` = ?"
	if err := m.conn.QueryRowCtx(ctx, &res, query, ticketId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customSlaTimerModel) Pause(ctx context.Context, tenantId, ticketId int64, pausedAt time.Time, reason string) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `sla_timer` set `paused_at` = ?, `pause_reason` = ? where `ticket_id` = ? and `tenant_id` = ? and `paused_at` is null",
		pausedAt, reason, ticketId, tenantId)
	return err
}

func (m *customSlaTimerModel) Resume(ctx context.Context, tenantId, ticketId int64, addMinutes int64, newResponseDue, newResolveDue time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `sla_timer` set `paused_at` = null, `pause_reason` = null, `pause_total_min` = `pause_total_min` + ?, `response_due_at` = ?, `resolve_due_at` = ? where `ticket_id` = ? and `tenant_id` = ? and `paused_at` is not null",
		addMinutes, newResponseDue, newResolveDue, ticketId, tenantId)
	return err
}

func (m *customSlaTimerModel) MarkResponded(ctx context.Context, tenantId, ticketId int64, at time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `sla_timer` set `responded_at` = ? where `ticket_id` = ? and `tenant_id` = ? and `responded_at` is null",
		at, ticketId, tenantId)
	return err
}

func (m *customSlaTimerModel) MarkResolved(ctx context.Context, tenantId, ticketId int64, at time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `sla_timer` set `resolved_at` = ? where `ticket_id` = ? and `tenant_id` = ? and `resolved_at` is null",
		at, ticketId, tenantId)
	return err
}

// MarkBreach column 仅允许 response_breach_at / resolve_breach_at（升级留痕）。
func (m *customSlaTimerModel) MarkBreach(ctx context.Context, tenantId, timerId int64, column string, at time.Time) (bool, error) {
	if column != "response_breach_at" && column != "resolve_breach_at" {
		return false, sqlx.ErrNotFound
	}
	res, err := m.conn.ExecCtx(ctx,
		"update `sla_timer` set `"+column+"` = ?, `escalate_level` = `escalate_level` + 1 where `timer_id` = ? and `tenant_id` = ? and `"+column+"` is null",
		at, timerId, tenantId)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (m *customSlaTimerModel) ScanDueBreaches(ctx context.Context, tenantId int64, now time.Time, limit int) ([]*SlaTimer, error) {
	var list []*SlaTimer
	// 计时中（未暂停）、响应或解决任一到期且尚未标记超时
	query := "select " + slaTimerRows + " from `sla_timer` where `tenant_id` = ? and `paused_at` is null and ((`responded_at` is null and `response_breach_at` is null and `response_due_at` < ?) or (`resolved_at` is null and `resolve_breach_at` is null and `resolve_due_at` < ?)) limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, now, now, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customSlaTimerModel) ScanDueBreachesAll(ctx context.Context, limit int) ([]*SlaTimer, error) {
	var list []*SlaTimer
	// tenantaudit:ignore SLA 违约 cron 跨租户扫描（sla_breach_scan，升级前按归属租户重建 ctx）
	query := "select " + slaTimerRows + " from `sla_timer` where `paused_at` is null and ((`responded_at` is null and `response_breach_at` is null and `response_due_at` < now()) or (`resolved_at` is null and `resolve_breach_at` is null and `resolve_due_at` < now())) limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, limit); err != nil {
		return nil, err
	}
	return list, nil
}
