// alert custom 覆写：聚合键查找/轮询增量扫描/CAS 状态迁移（ADR-08 租户过滤显式携带）。
package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertModel = (*customAlertModel)(nil)

type (
	// AlertModel 告警（FR-OPS-002/003/009/010）。
	AlertModel interface {
		alertModel
		// InsertTx 事务内插入。
		InsertTx(ctx context.Context, session sqlx.Session, data *Alert) error
		FindOneScoped(ctx context.Context, tenantId, alertId int64) (*Alert, error)
		// FindActiveForAgg 抑制窗口内同聚合键的活跃告警（聚合累计用，FR-OPS-002）。
		FindActiveForAgg(ctx context.Context, tenantId, stationId int64, source, metric, level string, since time.Time) (*Alert, error)
		// AggHitTx 命中聚合：count+1 / last_seen / last_value / devices 覆写。
		AggHitTx(ctx context.Context, session sqlx.Session, alertId int64, lastValue *float64, devices string, hitAt time.Time) error
		ListPage(ctx context.Context, tenantId int64, status, level, source string, stationId int64, limit, offset int) ([]*Alert, int64, error)
		// ListPending 轮询增量（id > since_id 有序扫描，FR-OPS-010）。
		ListPending(ctx context.Context, tenantId, stationId, sinceId int64, limit int) ([]*Alert, error)
		// CountOpen 未读计数（Redis miss 回填）。
		CountOpen(ctx context.Context, tenantId, stationId int64) (int64, error)
		// UpdateAck 确认（OPEN→ACKED 记录人/时刻）。
		UpdateAck(ctx context.Context, tenantId, alertId, uid int64) error
		// UpdateClose 关闭（前置校验在 logic；OPEN/ACKED→CLOSED）。
		UpdateClose(ctx context.Context, tenantId, alertId, ticketId int64, reason string) error
		// UpdateArchive 归档（CLOSED→ARCHIVED）。
		UpdateArchive(ctx context.Context, tenantId, alertId int64) error
		// MaxIdScoped 站点范围最大告警 ID（轮询起点对齐）。
		MaxIdScoped(ctx context.Context, tenantId, stationId int64) (int64, error)
	}

	customAlertModel struct {
		*defaultAlertModel
		conn sqlx.SqlConn
	}
)

// NewAlertModel 构造。
func NewAlertModel(conn sqlx.SqlConn) AlertModel {
	return &customAlertModel{defaultAlertModel: newAlertModel(conn), conn: conn}
}

// 活跃 = OPEN/ACKED（CLOSED/ARCHIVED 不再聚合）。
const alertActiveFilter = "`status` in ('OPEN','ACKED')"

func (m *customAlertModel) InsertTx(ctx context.Context, session sqlx.Session, data *Alert) error {
	query := "insert into `alert` (`alert_id`,`rule_id`,`level`,`source`,`metric`,`comparator`,`threshold`,`last_value`,`station_id`,`station_name`,`device_id`,`sn`,`title`,`content`,`status`,`aggregate_count`,`devices`,`first_seen_at`,`last_seen_at`,`tenant_id`) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.AlertId, data.RuleId, data.Level, data.Source, data.Metric, data.Comparator,
		data.Threshold, data.LastValue, data.StationId, data.StationName, data.DeviceId,
		data.Sn, data.Title, data.Content, data.Status, data.AggregateCount, data.Devices,
		data.FirstSeenAt, data.LastSeenAt, data.TenantId)
	return err
}

func (m *customAlertModel) FindOneScoped(ctx context.Context, tenantId, alertId int64) (*Alert, error) {
	var res Alert
	query := "select " + alertRows + " from `alert` where `alert_id` = ? and `tenant_id` = ?"
	if err := m.conn.QueryRowCtx(ctx, &res, query, alertId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customAlertModel) FindActiveForAgg(ctx context.Context, tenantId, stationId int64, source, metric, level string, since time.Time) (*Alert, error) {
	var res Alert
	query := "select " + alertRows + " from `alert` where `tenant_id` = ? and `station_id` = ? and `source` = ? and `metric` = ? and `level` = ? and " + alertActiveFilter + " and `last_seen_at` >= ? order by `last_seen_at` desc limit 1"
	if err := m.conn.QueryRowCtx(ctx, &res, query, tenantId, stationId, source, metric, level, since); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customAlertModel) AggHitTx(ctx context.Context, session sqlx.Session, alertId int64, lastValue *float64, devices string, hitAt time.Time) error {
	if lastValue != nil {
		_, err := session.ExecCtx(ctx,
			"update `alert` set `aggregate_count` = `aggregate_count` + 1, `last_seen_at` = ?, `last_value` = ?, `devices` = ? where `alert_id` = ?",
			hitAt, *lastValue, devices, alertId)
		return err
	}
	_, err := session.ExecCtx(ctx,
		"update `alert` set `aggregate_count` = `aggregate_count` + 1, `last_seen_at` = ?, `devices` = ? where `alert_id` = ?",
		hitAt, devices, alertId)
	return err
}

func (m *customAlertModel) ListPage(ctx context.Context, tenantId int64, status, level, source string, stationId int64, limit, offset int) ([]*Alert, int64, error) {
	where := "`tenant_id` = ?"
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if level != "" {
		where += " and `level` = ?"
		args = append(args, level)
	}
	if source != "" {
		where += " and `source` = ?"
		args = append(args, source)
	}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `alert` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Alert
	args = append(args, limit, offset)
	query := "select " + alertRows + " from `alert` where " + where + " order by `alert_id` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customAlertModel) ListPending(ctx context.Context, tenantId, stationId, sinceId int64, limit int) ([]*Alert, error) {
	where := "`tenant_id` = ? and `alert_id` > ?"
	args := []any{tenantId, sinceId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var list []*Alert
	query := "select " + alertRows + " from `alert` where " + where + " order by `alert_id` asc limit ?"
	args = append(args, limit)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customAlertModel) CountOpen(ctx context.Context, tenantId, stationId int64) (int64, error) {
	where := "`tenant_id` = ? and `status` = 'OPEN'"
	args := []any{tenantId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n, "select count(*) from `alert` where "+where, args...)
	return n, err
}

func (m *customAlertModel) UpdateAck(ctx context.Context, tenantId, alertId, uid int64) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `alert` set `status` = 'ACKED', `acked_by` = ?, `acked_at` = now(3) where `alert_id` = ? and `tenant_id` = ? and `status` = 'OPEN'",
		uid, alertId, tenantId)
	return err
}

func (m *customAlertModel) UpdateClose(ctx context.Context, tenantId, alertId, ticketId int64, reason string) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `alert` set `status` = 'CLOSED', `closed_ticket_id` = ?, `closed_reason` = ?, `closed_at` = now(3) where `alert_id` = ? and `tenant_id` = ? and `status` in ('OPEN','ACKED')",
		ticketId, reason, alertId, tenantId)
	return err
}

func (m *customAlertModel) UpdateArchive(ctx context.Context, tenantId, alertId int64) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `alert` set `status` = 'ARCHIVED', `archived_at` = now(3) where `alert_id` = ? and `tenant_id` = ? and `status` = 'CLOSED'",
		alertId, tenantId)
	return err
}

func (m *customAlertModel) MaxIdScoped(ctx context.Context, tenantId, stationId int64) (int64, error) {
	where := "`tenant_id` = ?"
	args := []any{tenantId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var n sql.NullInt64
	err := m.conn.QueryRowCtx(ctx, &n, "select max(`alert_id`) from `alert` where "+where, args...)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return n.Int64, nil
}
