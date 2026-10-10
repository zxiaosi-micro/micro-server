// device_db custom 覆写：cmd / firmware / ota_task / ota_device / device_topology（S6-01）。
package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ---- cmd ----

var _ CmdModel = (*customCmdModel)(nil)

type (
	CmdModel interface {
		cmdModel
		// InsertTx 事务内插入。
		InsertTx(ctx context.Context, session sqlx.Session, data *Cmd) error
		// FindOneScoped 租户内查询。
		FindOneScoped(ctx context.Context, tenantId, cmdId int64) (*Cmd, error)
		// CasStatusTx 指令状态机 CAS（PENDING→SENT / SENT→ACKED / SENT→FAILED）。
		CasStatusTx(ctx context.Context, session sqlx.Session, tenantId, cmdId int64, from, to, failReason string) (int64, error)
		// MarkAckedTx ACK 回执（重试次数同步清零语义由 logic 决定）。
		MarkAckedTx(ctx context.Context, session sqlx.Session, tenantId, cmdId int64, ackedAt time.Time) (int64, error)
		// FindRetryDue 重试扫描（无租户条件——cron 跨租户扫描，软删过滤保留；E16 命中 idx_cmd_retry_scan）。
		FindRetryDue(ctx context.Context, now time.Time, limit int) ([]*Cmd, error)
		// IncRetryTx 重试计数 + next_exec_at 推进；超限置 FAILED。
		IncRetryTx(ctx context.Context, session sqlx.Session, cmdId int64, nextExecAt time.Time) error
		// ListPage 指令列表。
		ListPage(ctx context.Context, tenantId, deviceId int64, sn, status string, limit, offset int) ([]*Cmd, int64, error)
		// FindOneByClientCmdId 调用方幂等键查询。
		FindOneByClientCmdId(ctx context.Context, tenantId int64, clientCmdId string) (*Cmd, error)
	}

	customCmdModel struct {
		*defaultCmdModel
		conn sqlx.SqlConn
	}
)

func NewCmdModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) CmdModel {
	return &customCmdModel{defaultCmdModel: newCmdModel(conn, c, opts...), conn: conn}
}

const cmdSoftFilter = "`deleted_at` is null"

func (m *customCmdModel) InsertTx(ctx context.Context, session sqlx.Session, data *Cmd) error {
	query := "insert into `cmd` (`cmd_id`,`device_id`,`sn`,`cmd_type`,`params`,`status`,`retry_count`,`max_retry`,`next_exec_at`,`client_cmd_id`,`operator`,`tenant_id`) values (?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.CmdId, data.DeviceId, data.Sn, data.CmdType, data.Params, data.Status,
		data.RetryCount, data.MaxRetry, data.NextExecAt, data.ClientCmdId, data.Operator, data.TenantId)
	return err
}

func (m *customCmdModel) FindOneScoped(ctx context.Context, tenantId, cmdId int64) (*Cmd, error) {
	var res Cmd
	query := "select " + cmdRows + " from `cmd` where `cmd_id` = ? and `tenant_id` = ? and " + cmdSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, cmdId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customCmdModel) CasStatusTx(ctx context.Context, session sqlx.Session, tenantId, cmdId int64, from, to, failReason string) (int64, error) {
	res, err := session.ExecCtx(ctx,
		"update `cmd` set `status`=?, `fail_reason`=? where `cmd_id`=? and `tenant_id`=? and `status`=? and "+cmdSoftFilter,
		to, failReason, cmdId, tenantId, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customCmdModel) MarkAckedTx(ctx context.Context, session sqlx.Session, tenantId, cmdId int64, ackedAt time.Time) (int64, error) {
	res, err := session.ExecCtx(ctx,
		"update `cmd` set `status`='ACKED', `acked_at`=? where `cmd_id`=? and `tenant_id`=? and `status` in ('PENDING','SENT') and "+cmdSoftFilter,
		ackedAt, cmdId, tenantId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customCmdModel) FindRetryDue(ctx context.Context, now time.Time, limit int) ([]*Cmd, error) {
	var list []*Cmd
	// QueryRowsCtx（切片扫描必须 QueryRowsCtx，QueryRowCtx 传 &[]*T 运行时报 unsupported unmarshal）；
	// SENT/PENDING 且 next_exec_at 到期 = TimingWheel 失联后的兜底重试目标。
	query := "select " + cmdRows + " from `cmd` where `status` in ('PENDING','SENT') and `next_exec_at` <= ? and " + cmdSoftFilter + " order by `next_exec_at` limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, now, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customCmdModel) IncRetryTx(ctx context.Context, session sqlx.Session, cmdId int64, nextExecAt time.Time) error {
	_, err := session.ExecCtx(ctx,
		"update `cmd` set `retry_count`=`retry_count`+1, `next_exec_at`=?, `status`='PENDING' where `cmd_id`=? and `retry_count`<`max_retry` and "+cmdSoftFilter,
		nextExecAt, cmdId)
	return err
}

func (m *customCmdModel) ListPage(ctx context.Context, tenantId, deviceId int64, sn, status string, limit, offset int) ([]*Cmd, int64, error) {
	where := "`tenant_id` = ? and " + cmdSoftFilter
	args := []any{tenantId}
	if deviceId > 0 {
		where += " and `device_id` = ?"
		args = append(args, deviceId)
	}
	if sn != "" {
		where += " and `sn` = ?"
		args = append(args, sn)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `cmd` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Cmd
	args = append(args, limit, offset)
	query := "select " + cmdRows + " from `cmd` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customCmdModel) FindOneByClientCmdId(ctx context.Context, tenantId int64, clientCmdId string) (*Cmd, error) {
	var res Cmd
	query := "select " + cmdRows + " from `cmd` where `tenant_id` = ? and `client_cmd_id` = ? and " + cmdSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, tenantId, clientCmdId); err != nil {
		return nil, err
	}
	return &res, nil
}

// ---- firmware ----

var _ FirmwareModel = (*customFirmwareModel)(nil)

type (
	FirmwareModel interface {
		firmwareModel
		InsertTx(ctx context.Context, session sqlx.Session, data *Firmware) error
		FindOneScoped(ctx context.Context, tenantId, firmwareId int64) (*Firmware, error)
		ListPage(ctx context.Context, tenantId int64, productKey string, limit, offset int) ([]*Firmware, int64, error)
	}

	customFirmwareModel struct {
		*defaultFirmwareModel
		conn sqlx.SqlConn
	}
)

func NewFirmwareModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) FirmwareModel {
	return &customFirmwareModel{defaultFirmwareModel: newFirmwareModel(conn, c, opts...), conn: conn}
}

const fwSoftFilter = "`deleted_at` is null"

func (m *customFirmwareModel) InsertTx(ctx context.Context, session sqlx.Session, data *Firmware) error {
	query := "insert into `firmware` (`firmware_id`,`product_key`,`version`,`file_url`,`file_size`,`sha256`,`signature`,`sign_alg`,`remark`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.FirmwareId, data.ProductKey, data.Version, data.FileUrl, data.FileSize,
		data.Sha256, data.Signature, data.SignAlg, data.Remark, data.TenantId, data.CreatedBy)
	return err
}

func (m *customFirmwareModel) FindOneScoped(ctx context.Context, tenantId, firmwareId int64) (*Firmware, error) {
	var res Firmware
	query := "select " + firmwareRows + " from `firmware` where `firmware_id` = ? and `tenant_id` = ? and " + fwSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, firmwareId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customFirmwareModel) ListPage(ctx context.Context, tenantId int64, productKey string, limit, offset int) ([]*Firmware, int64, error) {
	where := "`tenant_id` = ? and " + fwSoftFilter
	args := []any{tenantId}
	if productKey != "" {
		where += " and `product_key` = ?"
		args = append(args, productKey)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `firmware` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Firmware
	args = append(args, limit, offset)
	query := "select " + firmwareRows + " from `firmware` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ---- ota_task ----

var _ OtaTaskModel = (*customOtaTaskModel)(nil)

type (
	OtaTaskModel interface {
		otaTaskModel
		InsertTx(ctx context.Context, session sqlx.Session, data *OtaTask) error
		FindOneScoped(ctx context.Context, tenantId, taskId int64) (*OtaTask, error)
		// CasStatus 任务状态机（RUNNING→PAUSED 等）。
		CasStatus(ctx context.Context, tenantId, taskId int64, from, to, reason string) (int64, error)
		// IncProgressTx 进度推进（成功/失败计数；失败率判定在 logic 层）。
		IncProgressTx(ctx context.Context, session sqlx.Session, taskId int64, success, failed int) error
		// ListRunningOrPaused OTA 调度扫描（无租户条件，cron 跨租户）。
		ListRunningOrPaused(ctx context.Context, limit int) ([]*OtaTask, error)
		ListPage(ctx context.Context, tenantId int64, status, productKey string, limit, offset int) ([]*OtaTask, int64, error)
	}

	customOtaTaskModel struct {
		*defaultOtaTaskModel
		conn sqlx.SqlConn
	}
)

func NewOtaTaskModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OtaTaskModel {
	return &customOtaTaskModel{defaultOtaTaskModel: newOtaTaskModel(conn, c, opts...), conn: conn}
}

const otaTaskSoftFilter = "`deleted_at` is null"

func (m *customOtaTaskModel) InsertTx(ctx context.Context, session sqlx.Session, data *OtaTask) error {
	query := "insert into `ota_task` (`task_id`,`task_no`,`name`,`product_key`,`firmware_id`,`rollback_firmware_id`,`batch_size`,`fail_threshold_pct`,`status`,`total`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.TaskId, data.TaskNo, data.Name, data.ProductKey, data.FirmwareId, data.RollbackFirmwareId,
		data.BatchSize, data.FailThresholdPct, data.Status, data.Total, data.TenantId, data.CreatedBy)
	return err
}

func (m *customOtaTaskModel) FindOneScoped(ctx context.Context, tenantId, taskId int64) (*OtaTask, error) {
	var res OtaTask
	query := "select " + otaTaskRows + " from `ota_task` where `task_id` = ? and `tenant_id` = ? and " + otaTaskSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, taskId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customOtaTaskModel) CasStatus(ctx context.Context, tenantId, taskId int64, from, to, reason string) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"update `ota_task` set `status`=?, `fail_reason`=? where `task_id`=? and `tenant_id`=? and `status`=? and "+otaTaskSoftFilter,
		to, reason, taskId, tenantId, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customOtaTaskModel) IncProgressTx(ctx context.Context, session sqlx.Session, taskId int64, success, failed int) error {
	_, err := session.ExecCtx(ctx,
		"update `ota_task` set `success_count`=`success_count`+?, `fail_count`=`fail_count`+? where `task_id`=?",
		success, failed, taskId)
	return err
}

func (m *customOtaTaskModel) ListRunningOrPaused(ctx context.Context, limit int) ([]*OtaTask, error) {
	var list []*OtaTask
	query := "select " + otaTaskRows + " from `ota_task` where `status` in ('RUNNING','PAUSED') and " + otaTaskSoftFilter + " order by `updated_at` limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customOtaTaskModel) ListPage(ctx context.Context, tenantId int64, status, productKey string, limit, offset int) ([]*OtaTask, int64, error) {
	where := "`tenant_id` = ? and " + otaTaskSoftFilter
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if productKey != "" {
		where += " and `product_key` = ?"
		args = append(args, productKey)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `ota_task` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*OtaTask
	args = append(args, limit, offset)
	query := "select " + otaTaskRows + " from `ota_task` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ---- ota_device ----

var _ OtaDeviceModel = (*customOtaDeviceModel)(nil)

type (
	OtaDeviceModel interface {
		otaDeviceModel
		InsertTx(ctx context.Context, session sqlx.Session, data *OtaDevice) error
		// ListByTask 明细清单。
		ListByTask(ctx context.Context, tenantId, taskId int64) ([]*OtaDevice, error)
		// ListDispatchable 单轮下发扫描（task 内 PENDING/FAILED 可重试行；断点续传基线）。
		ListDispatchable(ctx context.Context, taskId int64, limit int) ([]*OtaDevice, error)
		// MarkSentTx 置 SENT + 关联 cmd。
		MarkSentTx(ctx context.Context, session sqlx.Session, id, cmdId int64, dispatchedAt time.Time) error
		// CasStatus 行状态机（SENT→SUCCESS / SENT→FAILED）。
		CasStatus(ctx context.Context, id int64, from, to, errMsg string) (int64, error)
		// CountByStatus 任务内状态计数。
		CountByStatus(ctx context.Context, taskId int64) (map[string]int, error)
		// ResetForRollback 回滚：SUCCESS/FAILED → PENDING（回滚固件重推）。
		ResetForRollback(ctx context.Context, taskId int64) (int64, error)
	}

	customOtaDeviceModel struct {
		*defaultOtaDeviceModel
		conn sqlx.SqlConn
	}
)

func NewOtaDeviceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OtaDeviceModel {
	return &customOtaDeviceModel{defaultOtaDeviceModel: newOtaDeviceModel(conn, c, opts...), conn: conn}
}

func (m *customOtaDeviceModel) InsertTx(ctx context.Context, session sqlx.Session, data *OtaDevice) error {
	query := "insert into `ota_device` (`id`,`task_id`,`device_id`,`sn`,`status`,`tenant_id`) values (?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.Id, data.TaskId, data.DeviceId, data.Sn, data.Status, data.TenantId)
	return err
}

func (m *customOtaDeviceModel) ListByTask(ctx context.Context, tenantId, taskId int64) ([]*OtaDevice, error) {
	var list []*OtaDevice
	query := "select " + otaDeviceRows + " from `ota_device` where `task_id` = ? and `tenant_id` = ? order by `id`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, taskId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customOtaDeviceModel) ListDispatchable(ctx context.Context, taskId int64, limit int) ([]*OtaDevice, error) {
	var list []*OtaDevice
	query := "select " + otaDeviceRows + " from `ota_device` where `task_id` = ? and `status` in ('PENDING','FAILED') and `retry_count` < 3 order by `id` limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, taskId, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customOtaDeviceModel) MarkSentTx(ctx context.Context, session sqlx.Session, id, cmdId int64, dispatchedAt time.Time) error {
	_, err := session.ExecCtx(ctx,
		"update `ota_device` set `status`='SENT', `cmd_id`=?, `dispatched_at`=? where `id`=?",
		cmdId, dispatchedAt, id)
	return err
}

func (m *customOtaDeviceModel) CasStatus(ctx context.Context, id int64, from, to, errMsg string) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"update `ota_device` set `status`=?, `error`=?, `finished_at`=? where `id`=? and `status`=?",
		to, errMsg, time.Now(), id, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customOtaDeviceModel) CountByStatus(ctx context.Context, taskId int64) (map[string]int, error) {
	var rows []struct {
		Status string `db:"status"`
		Cnt    int    `db:"cnt"`
	}
	query := "select `status`, count(*) as cnt from `ota_device` where `task_id` = ? group by `status`"
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, taskId); err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.Status] = r.Cnt
	}
	return out, nil
}

func (m *customOtaDeviceModel) ResetForRollback(ctx context.Context, taskId int64) (int64, error) {
	res, err := m.conn.ExecCtx(ctx,
		"update `ota_device` set `status`='PENDING', `retry_count`=0 where `task_id`=? and `status` in ('SUCCESS','FAILED','SENT')",
		taskId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- device_topology ----

var _ DeviceTopologyModel = (*customDeviceTopologyModel)(nil)

type (
	DeviceTopologyModel interface {
		deviceTopologyModel
		// SoftDeleteByDeviceTx 全量替换前软删旧树。
		SoftDeleteByDeviceTx(ctx context.Context, session sqlx.Session, tenantId, deviceId int64) error
		// InsertTx 事务内插入节点。
		InsertTx(ctx context.Context, session sqlx.Session, data *DeviceTopology) error
		// ListByDevice 拓扑树（含 parent 空根）。
		ListByDevice(ctx context.Context, tenantId, deviceId int64) ([]*DeviceTopology, error)
	}

	customDeviceTopologyModel struct {
		*defaultDeviceTopologyModel
		conn sqlx.SqlConn
	}
)

func NewDeviceTopologyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DeviceTopologyModel {
	return &customDeviceTopologyModel{defaultDeviceTopologyModel: newDeviceTopologyModel(conn, c, opts...), conn: conn}
}

func (m *customDeviceTopologyModel) SoftDeleteByDeviceTx(ctx context.Context, session sqlx.Session, tenantId, deviceId int64) error {
	_, err := session.ExecCtx(ctx,
		"update `device_topology` set `deleted_at`=? where `device_id`=? and `tenant_id`=? and `deleted_at` is null",
		time.Now(), deviceId, tenantId)
	return err
}

func (m *customDeviceTopologyModel) InsertTx(ctx context.Context, session sqlx.Session, data *DeviceTopology) error {
	query := "insert into `device_topology` (`node_id`,`device_id`,`parent_id`,`node_type`,`node_name`,`sort`,`tenant_id`) values (?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.NodeId, data.DeviceId, data.ParentId, data.NodeType, data.NodeName, data.Sort, data.TenantId)
	return err
}

func (m *customDeviceTopologyModel) ListByDevice(ctx context.Context, tenantId, deviceId int64) ([]*DeviceTopology, error) {
	var list []*DeviceTopology
	query := "select " + deviceTopologyRows + " from `device_topology` where `device_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `sort`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, deviceId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

var _ = time.Second
