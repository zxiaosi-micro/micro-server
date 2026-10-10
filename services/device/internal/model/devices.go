// device_db custom 覆写（ADR-08）：租户过滤显式携带 + 事务内方法 + 扫描查询。
// device / device_lifecycle_log（S6-01）。
package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DeviceModel = (*customDeviceModel)(nil)

type (
	// DeviceModel device 表 custom 接口。
	DeviceModel interface {
		deviceModel
		// InsertTx 事务内插入（tenant_id 必填）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Device) error
		// FindOneScoped 租户内主键查询（软删过滤）。
		FindOneScoped(ctx context.Context, tenantId, deviceId int64) (*Device, error)
		// FindOneBySnScoped 租户内 SN 查询。
		FindOneBySnScoped(ctx context.Context, tenantId int64, sn string) (*Device, error)
		// UpdateStatusTx 事务内更新（含 secret/party/order_no/activated_at 整行更新）。
		UpdateTx(ctx context.Context, session sqlx.Session, data *Device) error
		// CasStatusTx 条件状态迁移：affected==0 → ErrStatusConflict（FR-DEV-003 白名单校验在 logic 层）。
		CasStatusTx(ctx context.Context, session sqlx.Session, tenantId, deviceId int64, from, to string) (int64, error)
		// ListPage 列表（禁 SELECT *，显式列；NoCache）。
		ListPage(ctx context.Context, tenantId int64, keyword, status, productKey string, limit, offset int) ([]*Device, int64, error)
		// ListByOrderNoByTenant 按来源订单查询。
		ListByOrderNoByTenant(ctx context.Context, tenantId int64, orderNo string) ([]*Device, error)
		// ListActiveByProduct OTA 目标设备扫描（跨租户由调用方保证；无 tenant 条件，软删过滤保留）。
		ListActiveByProduct(ctx context.Context, productKey string, limit int) ([]*Device, error)
		// ListByIds 按主键批量（OTA 灰度清单）。
		ListByIds(ctx context.Context, ids []int64) ([]*Device, error)
	}

	customDeviceModel struct {
		*defaultDeviceModel
		conn sqlx.SqlConn
	}
)

// NewDeviceModel returns a model for the database table.
func NewDeviceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DeviceModel {
	return &customDeviceModel{
		defaultDeviceModel: newDeviceModel(conn, c, opts...),
		conn:               conn,
	}
}

const deviceSoftFilter = "`deleted_at` is null"

func (m *customDeviceModel) InsertTx(ctx context.Context, session sqlx.Session, data *Device) error {
	query := "insert into `device` (`device_id`,`sn`,`product_key`,`model`,`batch_no`,`device_secret`,`status`,`party_id`,`order_no`,`activated_at`,`tenant_id`,`created_by`,`updated_by`) values (?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.DeviceId, data.Sn, data.ProductKey, data.Model, data.BatchNo,
		data.DeviceSecret, data.Status, data.PartyId, data.OrderNo, data.ActivatedAt,
		data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customDeviceModel) FindOneScoped(ctx context.Context, tenantId, deviceId int64) (*Device, error) {
	var res Device
	query := "select " + deviceRows + " from `device` where `device_id` = ? and `tenant_id` = ? and " + deviceSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, deviceId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customDeviceModel) FindOneBySnScoped(ctx context.Context, tenantId int64, sn string) (*Device, error) {
	var res Device
	query := "select " + deviceRows + " from `device` where `sn` = ? and `tenant_id` = ? and " + deviceSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, sn, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customDeviceModel) UpdateTx(ctx context.Context, session sqlx.Session, data *Device) error {
	query := "update `device` set `sn`=?,`product_key`=?,`model`=?,`batch_no`=?,`device_secret`=?,`status`=?,`party_id`=?,`order_no`=?,`activated_at`=?,`updated_by`=? where `device_id`=? and `tenant_id`=? and " + deviceSoftFilter
	_, err := session.ExecCtx(ctx, query,
		data.Sn, data.ProductKey, data.Model, data.BatchNo, data.DeviceSecret,
		data.Status, data.PartyId, data.OrderNo, data.ActivatedAt, data.UpdatedBy,
		data.DeviceId, data.TenantId)
	return err
}

func (m *customDeviceModel) CasStatusTx(ctx context.Context, session sqlx.Session, tenantId, deviceId int64, from, to string) (int64, error) {
	res, err := session.ExecCtx(ctx,
		"update `device` set `status`=? where `device_id`=? and `tenant_id`=? and `status`=? and "+deviceSoftFilter,
		to, deviceId, tenantId, from)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customDeviceModel) ListPage(ctx context.Context, tenantId int64, keyword, status, productKey string, limit, offset int) ([]*Device, int64, error) {
	where := "`tenant_id` = ? and " + deviceSoftFilter
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`sn` like ? or `model` like ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if productKey != "" {
		where += " and `product_key` = ?"
		args = append(args, productKey)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"select count(*) from `device` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Device
	query := "select " + deviceRows + " from `device` where " + where + " order by `created_at` desc limit ? offset ?"
	args = append(args, limit, offset)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customDeviceModel) ListByOrderNoByTenant(ctx context.Context, tenantId int64, orderNo string) ([]*Device, error) {
	var list []*Device
	query := "select " + deviceRows + " from `device` where `tenant_id` = ? and `order_no` = ? and " + deviceSoftFilter + " order by `created_at`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, orderNo); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customDeviceModel) ListActiveByProduct(ctx context.Context, productKey string, limit int) ([]*Device, error) {
	var list []*Device
	query := "select " + deviceRows + " from `device` where `product_key` = ? and `status` = 'ACTIVATED' and " + deviceSoftFilter + " limit ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, productKey, limit); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customDeviceModel) ListByIds(ctx context.Context, ids []int64) ([]*Device, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []*Device
	query := "select " + deviceRows + " from `device` where `device_id` in (" + placeholders(len(ids)) + ") and " + deviceSoftFilter
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func placeholders(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s
}

// ---- device_lifecycle_log ----

var _ DeviceLifecycleLogModel = (*customDeviceLifecycleLogModel)(nil)

type (
	DeviceLifecycleLogModel interface {
		deviceLifecycleLogModel
		// InsertTx 事务内追加（幂等键 device_id+event_type+event_id，1062 由调用方吞掉跳过）。
		InsertTx(ctx context.Context, session sqlx.Session, data *DeviceLifecycleLog) error
		// ListByDevice 生命周期时间线。
		ListByDevice(ctx context.Context, tenantId, deviceId int64) ([]*DeviceLifecycleLog, error)
	}

	customDeviceLifecycleLogModel struct {
		*defaultDeviceLifecycleLogModel
		conn sqlx.SqlConn
	}
)

func NewDeviceLifecycleLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) DeviceLifecycleLogModel {
	return &customDeviceLifecycleLogModel{
		defaultDeviceLifecycleLogModel: newDeviceLifecycleLogModel(conn, c, opts...),
		conn:                           conn,
	}
}

func (m *customDeviceLifecycleLogModel) InsertTx(ctx context.Context, session sqlx.Session, data *DeviceLifecycleLog) error {
	query := "insert into `device_lifecycle_log` (`log_id`,`device_id`,`sn`,`from_status`,`to_status`,`event_type`,`event_id`,`remark`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.LogId, data.DeviceId, data.Sn, data.FromStatus, data.ToStatus,
		data.EventType, data.EventId, data.Remark, data.TenantId, data.CreatedBy)
	return err
}

func (m *customDeviceLifecycleLogModel) ListByDevice(ctx context.Context, tenantId, deviceId int64) ([]*DeviceLifecycleLog, error) {
	var list []*DeviceLifecycleLog
	query := "select " + deviceLifecycleLogRows + " from `device_lifecycle_log` where `device_id` = ? and `tenant_id` = ? order by `created_at`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, deviceId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

var _ = time.Second
