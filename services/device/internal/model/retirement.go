// retirement 三表 custom 覆写（ADR-08）：租户过滤显式携带 + 事务内方法（S10-01 退役评估/回收）。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ---- retirement_assessment ----

var _ RetirementAssessmentModel = (*customRetirementAssessmentModel)(nil)

type (
	// RetirementAssessmentModel 退役评估 custom 接口。
	RetirementAssessmentModel interface {
		retirementAssessmentModel
		// InsertTx 事务内插入（tenant_id 必填）。
		InsertTx(ctx context.Context, session sqlx.Session, data *RetirementAssessment) error
		// FindOneScoped 租户内主键查询（软删过滤）。
		FindOneScoped(ctx context.Context, tenantId, assessmentId int64) (*RetirementAssessment, error)
		// UpdateTx 事务内更新（检测结果/评级/处置）。
		UpdateTx(ctx context.Context, session sqlx.Session, data *RetirementAssessment) error
		// NextRoundByDevice 下一评估轮次（MAX(round)+1，无历史=1）。
		NextRoundByDevice(ctx context.Context, tenantId, deviceId int64) (int64, error)
		// HasPendingByDevice 是否存在未完结评估（disposition=PENDING，防重复发起）。
		HasPendingByDevice(ctx context.Context, tenantId, deviceId int64) (bool, error)
		// ListPage 列表（禁 SELECT *，显式列；NoCache）。
		ListPage(ctx context.Context, tenantId int64, keyword, grade, disposition string, limit, offset int) ([]*RetirementAssessment, int64, error)
	}

	customRetirementAssessmentModel struct {
		*defaultRetirementAssessmentModel
		conn sqlx.SqlConn
	}
)

// NewRetirementAssessmentModel returns a model for the database table.
func NewRetirementAssessmentModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RetirementAssessmentModel {
	return &customRetirementAssessmentModel{
		defaultRetirementAssessmentModel: newRetirementAssessmentModel(conn, c, opts...),
		conn:                             conn,
	}
}

const raSoftFilter = "`deleted_at` is null"

func (m *customRetirementAssessmentModel) InsertTx(ctx context.Context, session sqlx.Session, data *RetirementAssessment) error {
	query := "insert into `retirement_assessment` (`assessment_id`,`assessment_no`,`device_id`,`sn`,`round`,`soh`,`cycle_count`,`resistance_mohm`,`consistency_ok`,`safety_json`,`grade`,`disposition`,`inspection_ticket_no`,`remark`,`tenant_id`,`created_by`,`updated_by`) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.AssessmentId, data.AssessmentNo, data.DeviceId, data.Sn, data.Round,
		data.Soh, data.CycleCount, data.ResistanceMohm, data.ConsistencyOk, data.SafetyJson,
		data.Grade, data.Disposition, data.InspectionTicketNo, data.Remark,
		data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customRetirementAssessmentModel) FindOneScoped(ctx context.Context, tenantId, assessmentId int64) (*RetirementAssessment, error) {
	var res RetirementAssessment
	query := "select " + retirementAssessmentRows + " from `retirement_assessment` where `assessment_id` = ? and `tenant_id` = ? and " + raSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, assessmentId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customRetirementAssessmentModel) UpdateTx(ctx context.Context, session sqlx.Session, data *RetirementAssessment) error {
	query := "update `retirement_assessment` set `soh`=?,`cycle_count`=?,`resistance_mohm`=?,`consistency_ok`=?,`safety_json`=?,`grade`=?,`disposition`=?,`updated_by`=? where `assessment_id`=? and `tenant_id`=? and " + raSoftFilter
	_, err := session.ExecCtx(ctx, query,
		data.Soh, data.CycleCount, data.ResistanceMohm, data.ConsistencyOk, data.SafetyJson,
		data.Grade, data.Disposition, data.UpdatedBy,
		data.AssessmentId, data.TenantId)
	return err
}

func (m *customRetirementAssessmentModel) NextRoundByDevice(ctx context.Context, tenantId, deviceId int64) (int64, error) {
	var next int64
	query := "select ifnull(max(`round`), 0) + 1 from `retirement_assessment` where `device_id` = ? and `tenant_id` = ? and " + raSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &next, query, deviceId, tenantId); err != nil {
		return 0, err
	}
	return next, nil
}

func (m *customRetirementAssessmentModel) HasPendingByDevice(ctx context.Context, tenantId, deviceId int64) (bool, error) {
	var count int64
	query := "select count(*) from `retirement_assessment` where `device_id` = ? and `tenant_id` = ? and `disposition` = 'PENDING' and " + raSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &count, query, deviceId, tenantId); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *customRetirementAssessmentModel) ListPage(ctx context.Context, tenantId int64, keyword, grade, disposition string, limit, offset int) ([]*RetirementAssessment, int64, error) {
	where := "`tenant_id` = ? and " + raSoftFilter
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`assessment_no` like ? or `sn` like ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	if grade != "" {
		where += " and `grade` = ?"
		args = append(args, grade)
	}
	if disposition != "" {
		where += " and `disposition` = ?"
		args = append(args, disposition)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"select count(*) from `retirement_assessment` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*RetirementAssessment
	query := "select " + retirementAssessmentRows + " from `retirement_assessment` where " + where + " order by `created_at` desc limit ? offset ?"
	args = append(args, limit, offset)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ---- device_soh_snapshot ----

var _ DeviceSohSnapshotModel = (*customDeviceSohSnapshotModel)(nil)

type (
	// DeviceSohSnapshotModel SOH 快照 custom 接口。
	DeviceSohSnapshotModel interface {
		deviceSohSnapshotModel
		// InsertTx 事务内插入（检测结果留痕）。
		InsertTx(ctx context.Context, session sqlx.Session, data *DeviceSohSnapshot) error
	}

	customDeviceSohSnapshotModel struct {
		*defaultDeviceSohSnapshotModel
		conn sqlx.SqlConn
	}
)

// NewDeviceSohSnapshotModel returns a model for the database table.
func NewDeviceSohSnapshotModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DeviceSohSnapshotModel {
	return &customDeviceSohSnapshotModel{
		defaultDeviceSohSnapshotModel: newDeviceSohSnapshotModel(conn, c, opts...),
		conn:                          conn,
	}
}

func (m *customDeviceSohSnapshotModel) InsertTx(ctx context.Context, session sqlx.Session, data *DeviceSohSnapshot) error {
	query := "insert into `device_soh_snapshot` (`snapshot_id`,`assessment_id`,`device_id`,`sn`,`soh`,`cycle_count`,`resistance_mohm`,`min_cell_v_mv`,`max_cell_v_mv`,`max_temp_dc`,`measured_at`,`source`,`tenant_id`) values (?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.SnapshotId, data.AssessmentId, data.DeviceId, data.Sn,
		data.Soh, data.CycleCount, data.ResistanceMohm, data.MinCellVMv, data.MaxCellVMv, data.MaxTempDc,
		data.MeasuredAt, data.Source, data.TenantId)
	return err
}

// ---- recycle_order ----

var _ RecycleOrderModel = (*customRecycleOrderModel)(nil)

type (
	// RecycleOrderModel 回收单 custom 接口。
	RecycleOrderModel interface {
		recycleOrderModel
		// InsertTx 事务内插入（trace_code 全局唯一）。
		InsertTx(ctx context.Context, session sqlx.Session, data *RecycleOrder) error
		// FindOneScoped 租户内主键查询（软删过滤）。
		FindOneScoped(ctx context.Context, tenantId, recycleId int64) (*RecycleOrder, error)
		// FindOneByNoScoped 租户内单号查询。
		FindOneByNoScoped(ctx context.Context, tenantId int64, recycleNo string) (*RecycleOrder, error)
		// UpdateTx 事务内更新（凭证/状态/回收商）。
		UpdateTx(ctx context.Context, session sqlx.Session, data *RecycleOrder) error
		// ListPage 列表（回收单号/SN/溯源码模糊）。
		ListPage(ctx context.Context, tenantId int64, keyword, status string, limit, offset int) ([]*RecycleOrder, int64, error)
	}

	customRecycleOrderModel struct {
		*defaultRecycleOrderModel
		conn sqlx.SqlConn
	}
)

// NewRecycleOrderModel returns a model for the database table.
func NewRecycleOrderModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) RecycleOrderModel {
	return &customRecycleOrderModel{
		defaultRecycleOrderModel: newRecycleOrderModel(conn, c, opts...),
		conn:                     conn,
	}
}

const roSoftFilter = "`deleted_at` is null"

func (m *customRecycleOrderModel) InsertTx(ctx context.Context, session sqlx.Session, data *RecycleOrder) error {
	query := "insert into `recycle_order` (`recycle_id`,`recycle_no`,`assessment_id`,`device_id`,`sn`,`recycler_party_id`,`residual_value`,`trace_code`,`evidence_file_id`,`status`,`remark`,`tenant_id`,`created_by`,`updated_by`) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.RecycleId, data.RecycleNo, data.AssessmentId, data.DeviceId, data.Sn,
		data.RecyclerPartyId, data.ResidualValue, data.TraceCode, data.EvidenceFileId,
		data.Status, data.Remark, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customRecycleOrderModel) FindOneScoped(ctx context.Context, tenantId, recycleId int64) (*RecycleOrder, error) {
	var res RecycleOrder
	query := "select " + recycleOrderRows + " from `recycle_order` where `recycle_id` = ? and `tenant_id` = ? and " + roSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, recycleId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customRecycleOrderModel) FindOneByNoScoped(ctx context.Context, tenantId int64, recycleNo string) (*RecycleOrder, error) {
	var res RecycleOrder
	query := "select " + recycleOrderRows + " from `recycle_order` where `recycle_no` = ? and `tenant_id` = ? and " + roSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, recycleNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customRecycleOrderModel) UpdateTx(ctx context.Context, session sqlx.Session, data *RecycleOrder) error {
	query := "update `recycle_order` set `recycler_party_id`=?,`residual_value`=?,`evidence_file_id`=?,`status`=?,`updated_by`=? where `recycle_id`=? and `tenant_id`=? and " + roSoftFilter
	_, err := session.ExecCtx(ctx, query,
		data.RecyclerPartyId, data.ResidualValue, data.EvidenceFileId, data.Status, data.UpdatedBy,
		data.RecycleId, data.TenantId)
	return err
}

func (m *customRecycleOrderModel) ListPage(ctx context.Context, tenantId int64, keyword, status string, limit, offset int) ([]*RecycleOrder, int64, error) {
	where := "`tenant_id` = ? and " + roSoftFilter
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`recycle_no` like ? or `sn` like ? or `trace_code` like ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw, kw)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"select count(*) from `recycle_order` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*RecycleOrder
	query := "select " + recycleOrderRows + " from `recycle_order` where " + where + " order by `created_at` desc limit ? offset ?"
	args = append(args, limit, offset)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
