// ticket custom 覆写：状态机 CAS/派单更新/超时扫描/轮询增量（ADR-08 + ADR-09）。
package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TicketModel = (*customTicketModel)(nil)

type (
	// TicketModel 工单（FR-OPS-004/005/008/010）。
	TicketModel interface {
		ticketModel
		// InsertTx 事务内插入（client_action_id/工单号冲突 = 幂等重放，调用方查回）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Ticket) error
		FindOneScoped(ctx context.Context, tenantId, ticketId int64) (*Ticket, error)
		FindByClientActionScoped(ctx context.Context, tenantId int64, clientActionId string) (*Ticket, error)
		ListPage(ctx context.Context, tenantId int64, status, ticketType string, stationId, assigneeUid, partyId int64, limit, offset int) ([]*Ticket, int64, error)
		// ListPending 轮询增量（id > since_id，FR-OPS-010）。
		ListPending(ctx context.Context, tenantId, stationId, assigneeUid int64, status string, sinceId int64, limit int) ([]*Ticket, error)
		// CountActive 过滤条件下活跃（非终态）计数（轮询 unread）。
		CountActive(ctx context.Context, tenantId, stationId, assigneeUid int64, status string) (int64, error)
		// UpdateDispatch 派单（PENDING_DISPATCH→DISPATCHED，CAS 守卫；deadline 写入）。
		UpdateDispatchTx(ctx context.Context, session sqlx.Session, tenantId, ticketId, assigneeUid int64, assigneeName string, dispatchAt, deadline time.Time) (bool, error)
		// CasStatus 通用状态守卫迁移（from → to；附时刻列可选）。
		CasStatus(ctx context.Context, tenantId, ticketId int64, from, to string) (bool, error)
		// UpdateEscalate 接单超时升级（escalate_level+1、deadline 顺延、受理人清空回待派）。
		UpdateEscalate(ctx context.Context, tenantId, ticketId int64, newDeadline time.Time) error
		// ScanAcceptTimeout 接单超时扫描（单租户，ADR-09）。
		ScanAcceptTimeout(ctx context.Context, tenantId int64, before time.Time, limit int) ([]*Ticket, error)
		// ScanAcceptTimeoutAll 跨租户平台扫描（cron 用，SQL 无租户条件）。
		ScanAcceptTimeoutAll(ctx context.Context, before time.Time, limit int) ([]*Ticket, error)
		// UpdateCloseFields 验收通过关单/取消留因。
		UpdateCloseFields(ctx context.Context, tenantId, ticketId int64, fields map[string]any) error
		// MaxIdScoped 轮询起点对齐。
		MaxIdScoped(ctx context.Context, tenantId, stationId int64) (int64, error)
	}

	customTicketModel struct {
		*defaultTicketModel
		conn sqlx.SqlConn
	}
)

// NewTicketModel 构造。
func NewTicketModel(conn sqlx.SqlConn) TicketModel {
	return &customTicketModel{defaultTicketModel: newTicketModel(conn), conn: conn}
}

const ticketSoftFilter = "`deleted_at` is null"

// nullInt64 int64 → sql.NullInt64（0=NULL：party_id 等可空外键的统一口径）。
func nullInt64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v > 0} }

// 终态集合（轮询 unread 排除）。
const ticketActiveFilter = "`status` not in ('CLOSED','CANCELLED')"

func (m *customTicketModel) InsertTx(ctx context.Context, session sqlx.Session, data *Ticket) error {
	query := "insert into `ticket` (`ticket_id`,`ticket_no`,`type`,`title`,`content`,`priority`,`station_id`,`station_name`,`device_id`,`sn`,`alert_id`,`inspection_item_id`,`assignee_uid`,`assignee_name`,`status`,`escalate_level`,`created_by`,`client_action_id`,`party_id`,`tenant_id`) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.TicketId, data.TicketNo, data.Type, data.Title, data.Content, data.Priority,
		data.StationId, data.StationName, data.DeviceId, data.Sn, data.AlertId,
		data.InspectionItemId, data.AssigneeUid, data.AssigneeName, data.Status,
		data.EscalateLevel, data.CreatedBy, data.ClientActionId, data.PartyId, data.TenantId)
	return err
}

func (m *customTicketModel) FindOneScoped(ctx context.Context, tenantId, ticketId int64) (*Ticket, error) {
	var res Ticket
	query := "select " + ticketRows + " from `ticket` where `ticket_id` = ? and `tenant_id` = ? and " + ticketSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, ticketId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customTicketModel) FindByClientActionScoped(ctx context.Context, tenantId int64, clientActionId string) (*Ticket, error) {
	var res Ticket
	query := "select " + ticketRows + " from `ticket` where `tenant_id` = ? and `client_action_id` = ? and " + ticketSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, tenantId, clientActionId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customTicketModel) ListPage(ctx context.Context, tenantId int64, status, ticketType string, stationId, assigneeUid, partyId int64, limit, offset int) ([]*Ticket, int64, error) {
	where := "`tenant_id` = ? and " + ticketSoftFilter
	args := []any{tenantId}
	if partyId > 0 {
		where += " and `party_id` = ?"
		args = append(args, partyId)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if ticketType != "" {
		where += " and `type` = ?"
		args = append(args, ticketType)
	}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	if assigneeUid > 0 {
		where += " and `assignee_uid` = ?"
		args = append(args, assigneeUid)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `ticket` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Ticket
	args = append(args, limit, offset)
	query := "select " + ticketRows + " from `ticket` where " + where + " order by `ticket_id` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customTicketModel) ListPending(ctx context.Context, tenantId, stationId, assigneeUid int64, status string, sinceId int64, limit int) ([]*Ticket, error) {
	where := "`tenant_id` = ? and `ticket_id` > ? and " + ticketSoftFilter
	args := []any{tenantId, sinceId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	if assigneeUid > 0 {
		where += " and `assignee_uid` = ?"
		args = append(args, assigneeUid)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var list []*Ticket
	query := "select " + ticketRows + " from `ticket` where " + where + " order by `ticket_id` asc limit ?"
	args = append(args, limit)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customTicketModel) CountActive(ctx context.Context, tenantId, stationId, assigneeUid int64, status string) (int64, error) {
	where := "`tenant_id` = ? and " + ticketSoftFilter
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	} else {
		where += " and " + ticketActiveFilter
	}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	if assigneeUid > 0 {
		where += " and `assignee_uid` = ?"
		args = append(args, assigneeUid)
	}
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n, "select count(*) from `ticket` where "+where, args...)
	return n, err
}

func (m *customTicketModel) UpdateDispatchTx(ctx context.Context, session sqlx.Session, tenantId, ticketId, assigneeUid int64, assigneeName string, dispatchAt, deadline time.Time) (bool, error) {
	res, err := session.ExecCtx(ctx,
		"update `ticket` set `assignee_uid` = ?, `assignee_name` = ?, `status` = 'DISPATCHED', `dispatch_at` = ?, `accept_deadline` = ? where `ticket_id` = ? and `tenant_id` = ? and `status` = 'PENDING_DISPATCH'",
		assigneeUid, assigneeName, dispatchAt, deadline, ticketId, tenantId)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (m *customTicketModel) CasStatus(ctx context.Context, tenantId, ticketId int64, from, to string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx,
		"update `ticket` set `status` = ? where `ticket_id` = ? and `tenant_id` = ? and `status` = ?", to, ticketId, tenantId, from)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (m *customTicketModel) UpdateEscalate(ctx context.Context, tenantId, ticketId int64, newDeadline time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `ticket` set `escalate_level` = `escalate_level` + 1, `accept_deadline` = ?, `dispatch_at` = now(3) where `ticket_id` = ? and `tenant_id` = ? and `status` = 'DISPATCHED' and `accept_deadline` < now(3)",
		newDeadline, ticketId, tenantId)
	return err
}

func (m *customTicketModel) ScanAcceptTimeout(ctx context.Context, tenantId int64, before time.Time, limit int) ([]*Ticket, error) {
	var list []*Ticket
	query := "select " + ticketRows + " from `ticket` where `tenant_id` = ? and `status` = 'DISPATCHED' and `accept_deadline` is not null and `accept_deadline` < ? and " + ticketSoftFilter + " order by `accept_deadline` asc limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, before, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customTicketModel) ScanAcceptTimeoutAll(ctx context.Context, before time.Time, limit int) ([]*Ticket, error) {
	var list []*Ticket
	// tenantaudit:ignore 工单受理超时 cron 跨租户扫描（ticket_accept_timeout，升级前按归属租户重建 ctx）
	query := "select " + ticketRows + " from `ticket` where `status` = 'DISPATCHED' and `accept_deadline` is not null and `accept_deadline` < ? and " + ticketSoftFilter + " order by `accept_deadline` asc limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, before, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customTicketModel) UpdateCloseFields(ctx context.Context, tenantId, ticketId int64, fields map[string]any) error {
	set, args := buildSet(fields)
	query := "update `ticket` set " + set + " where `ticket_id` = ? and `tenant_id` = ? and " + ticketSoftFilter
	args = append(args, ticketId, tenantId)
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customTicketModel) MaxIdScoped(ctx context.Context, tenantId, stationId int64) (int64, error) {
	where := "`tenant_id` = ? and " + ticketSoftFilter
	args := []any{tenantId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var n sql.NullInt64
	err := m.conn.QueryRowCtx(ctx, &n, "select max(`ticket_id`) from `ticket` where "+where, args...)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return n.Int64, nil
}
