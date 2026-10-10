// inspection_task/item custom 覆写：cron 生成幂等 + 批量提交 + 异常项回填工单（FR-OPS-007）。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ InspectionTaskModel = (*customInspectionTaskModel)(nil)

type (
	// InspectionTaskModel 巡检任务。
	InspectionTaskModel interface {
		inspectionTaskModel
		// InsertTx 事务内插入（uk_itask_plan_sched 冲突 = cron 重放，调用方吞掉）。
		InsertTx(ctx context.Context, session sqlx.Session, data *InspectionTask) error
		FindOneScoped(ctx context.Context, tenantId, taskId int64) (*InspectionTask, error)
		ListPage(ctx context.Context, tenantId, stationId int64, status string, limit, offset int) ([]*InspectionTask, int64, error)
		// CasSubmit 提交（PENDING→SUBMITTED，CAS 守卫；回填提交人/时刻/异常数/工单）。
		CasSubmit(ctx context.Context, tenantId, taskId, uid int64, location, photos, remark string, anomalyCount int64, ticketIds string) (bool, error)
	}

	customInspectionTaskModel struct {
		*defaultInspectionTaskModel
		conn sqlx.SqlConn
	}
)

// NewInspectionTaskModel 构造。
func NewInspectionTaskModel(conn sqlx.SqlConn) InspectionTaskModel {
	return &customInspectionTaskModel{defaultInspectionTaskModel: newInspectionTaskModel(conn), conn: conn}
}

func (m *customInspectionTaskModel) InsertTx(ctx context.Context, session sqlx.Session, data *InspectionTask) error {
	query := "insert into `inspection_task` (`task_id`,`plan_id`,`template_id`,`station_id`,`station_name`,`title`,`scheduled_for`,`status`,`tenant_id`) values (?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.TaskId, data.PlanId, data.TemplateId, data.StationId, data.StationName,
		data.Title, data.ScheduledFor, data.Status, data.TenantId)
	return err
}

func (m *customInspectionTaskModel) FindOneScoped(ctx context.Context, tenantId, taskId int64) (*InspectionTask, error) {
	var res InspectionTask
	query := "select " + inspectionTaskRows + " from `inspection_task` where `task_id` = ? and `tenant_id` = ?"
	if err := m.conn.QueryRowCtx(ctx, &res, query, taskId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInspectionTaskModel) ListPage(ctx context.Context, tenantId, stationId int64, status string, limit, offset int) ([]*InspectionTask, int64, error) {
	where := "`tenant_id` = ?"
	args := []any{tenantId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `inspection_task` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*InspectionTask
	args = append(args, limit, offset)
	query := "select " + inspectionTaskRows + " from `inspection_task` where " + where + " order by `task_id` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customInspectionTaskModel) CasSubmit(ctx context.Context, tenantId, taskId, uid int64, location, photos, remark string, anomalyCount int64, ticketIds string) (bool, error) {
	// photos/ticket_ids 为 JSON 列：空串非合法 JSON，NULLIF 置 NULL（3140 防御）
	res, err := m.conn.ExecCtx(ctx,
		"update `inspection_task` set `status` = 'SUBMITTED', `submitted_by` = ?, `submitted_at` = now(3), `location` = ?, `photos` = nullif(?, ''), `remark` = ?, `anomaly_count` = ?, `ticket_ids` = nullif(?, '') where `task_id` = ? and `tenant_id` = ? and `status` = 'PENDING'",
		uid, location, photos, remark, anomalyCount, ticketIds, taskId, tenantId)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
