// inspection_plan custom 覆写：station_created 幂等建计划 + 到期扫描（ADR-09）。
package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ InspectionPlanModel = (*customInspectionPlanModel)(nil)

type (
	// InspectionPlanModel 巡检计划（cron 简化集合）。
	InspectionPlanModel interface {
		inspectionPlanModel
		// InsertTx 事务内插入（uk_iplan_station_name 冲突 = station_created 重放，调用方吞掉）。
		InsertTx(ctx context.Context, session sqlx.Session, data *InspectionPlan) error
		FindOneScoped(ctx context.Context, tenantId, planId int64) (*InspectionPlan, error)
		// FindByStationNameTx 同站同名计划（重放判定）。
		FindByStationNameTx(ctx context.Context, session sqlx.Session, tenantId, stationId int64, name string) (*InspectionPlan, error)
		ListPage(ctx context.Context, tenantId, stationId int64, limit, offset int) ([]*InspectionPlan, int64, error)
		UpdateFields(ctx context.Context, tenantId, planId int64, fields map[string]any) error
		// ScanDue 到期待生成计划（单租户，ADR-09 扫描源）。
		ScanDue(ctx context.Context, tenantId int64, now time.Time, limit int) ([]*InspectionPlan, error)
		// ScanDueAll 跨租户平台扫描（cron 用）。
		ScanDueAll(ctx context.Context, now time.Time, limit int) ([]*InspectionPlan, error)
		// MarkExeced 生成任务后推进 next_exec_at / last_exec_at。
		MarkExeced(ctx context.Context, tenantId, planId int64, lastExec, nextExec time.Time) error
		SoftDelete(ctx context.Context, tenantId, planId int64) error
	}

	customInspectionPlanModel struct {
		*defaultInspectionPlanModel
		conn sqlx.SqlConn
	}
)

// NewInspectionPlanModel 构造。
func NewInspectionPlanModel(conn sqlx.SqlConn) InspectionPlanModel {
	return &customInspectionPlanModel{defaultInspectionPlanModel: newInspectionPlanModel(conn), conn: conn}
}

const inspectionPlanSoftFilter = "`deleted_at` is null"

func (m *customInspectionPlanModel) InsertTx(ctx context.Context, session sqlx.Session, data *InspectionPlan) error {
	query := "insert into `inspection_plan` (`plan_id`,`station_id`,`station_name`,`name`,`template_id`,`cron_expr`,`enabled`,`next_exec_at`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.PlanId, data.StationId, data.StationName, data.Name, data.TemplateId,
		data.CronExpr, data.Enabled, data.NextExecAt, data.TenantId, data.CreatedBy)
	return err
}

func (m *customInspectionPlanModel) FindOneScoped(ctx context.Context, tenantId, planId int64) (*InspectionPlan, error) {
	var res InspectionPlan
	query := "select " + inspectionPlanRows + " from `inspection_plan` where `plan_id` = ? and `tenant_id` = ? and " + inspectionPlanSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, planId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInspectionPlanModel) FindByStationNameTx(ctx context.Context, session sqlx.Session, tenantId, stationId int64, name string) (*InspectionPlan, error) {
	var res InspectionPlan
	query := "select " + inspectionPlanRows + " from `inspection_plan` where `tenant_id` = ? and `station_id` = ? and `name` = ? and " + inspectionPlanSoftFilter + " limit 1"
	if err := session.QueryRowCtx(ctx, &res, query, tenantId, stationId, name); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInspectionPlanModel) ListPage(ctx context.Context, tenantId, stationId int64, limit, offset int) ([]*InspectionPlan, int64, error) {
	where := "`tenant_id` = ? and " + inspectionPlanSoftFilter
	args := []any{tenantId}
	if stationId > 0 {
		where += " and `station_id` = ?"
		args = append(args, stationId)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `inspection_plan` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*InspectionPlan
	args = append(args, limit, offset)
	query := "select " + inspectionPlanRows + " from `inspection_plan` where " + where + " order by `plan_id` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customInspectionPlanModel) UpdateFields(ctx context.Context, tenantId, planId int64, fields map[string]any) error {
	set, args := buildSet(fields)
	query := "update `inspection_plan` set " + set + " where `plan_id` = ? and `tenant_id` = ? and " + inspectionPlanSoftFilter
	args = append(args, planId, tenantId)
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customInspectionPlanModel) ScanDue(ctx context.Context, tenantId int64, now time.Time, limit int) ([]*InspectionPlan, error) {
	var list []*InspectionPlan
	query := "select " + inspectionPlanRows + " from `inspection_plan` where `tenant_id` = ? and `enabled` = 1 and `next_exec_at` <= ? and " + inspectionPlanSoftFilter + " order by `next_exec_at` asc limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, now, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customInspectionPlanModel) ScanDueAll(ctx context.Context, now time.Time, limit int) ([]*InspectionPlan, error) {
	var list []*InspectionPlan
	// tenantaudit:ignore 巡检计划生成 cron 跨租户扫描（inspection_task_generate，生成前按归属租户重建 ctx）
	query := "select " + inspectionPlanRows + " from `inspection_plan` where `enabled` = 1 and `next_exec_at` <= ? and " + inspectionPlanSoftFilter + " order by `next_exec_at` asc limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, now, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customInspectionPlanModel) MarkExeced(ctx context.Context, tenantId, planId int64, lastExec, nextExec time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `inspection_plan` set `last_exec_at` = ?, `next_exec_at` = ? where `plan_id` = ? and `tenant_id` = ?",
		lastExec, nextExec, planId, tenantId)
	return err
}

func (m *customInspectionPlanModel) SoftDelete(ctx context.Context, tenantId, planId int64) error {
	_, err := m.conn.ExecCtx(ctx,
		"update `inspection_plan` set `deleted_at` = now(), `enabled` = 0 where `plan_id` = ? and `tenant_id` = ? and "+inspectionPlanSoftFilter,
		planId, tenantId)
	return err
}
