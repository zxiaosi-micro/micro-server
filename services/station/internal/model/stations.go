// station_db custom 覆写（ADR-08）：租户过滤显式携带 + 事务内方法 + 幂等重放查询。
package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ---- station ----

var _ StationModel = (*customStationModel)(nil)

type (
	StationModel interface {
		stationModel
		// InsertTx 事务内插入。
		InsertTx(ctx context.Context, session sqlx.Session, data *Station) error
		FindOneScoped(ctx context.Context, tenantId, stationId int64) (*Station, error)
		// FindOneByOrderNoScoped Saga 幂等重放查询（uk_station_tenant_order）。
		FindOneByOrderNoScoped(ctx context.Context, tenantId int64, orderNo string) (*Station, error)
		// FindOneByNoScoped 场站编号查询。
		FindOneByNoScoped(ctx context.Context, tenantId int64, stationNo string) (*Station, error)
		ListPage(ctx context.Context, tenantId int64, keyword, status, stationType string, limit, offset int) ([]*Station, int64, error)
	}

	customStationModel struct {
		*defaultStationModel
		conn sqlx.SqlConn
	}
)

func NewStationModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationModel {
	return &customStationModel{defaultStationModel: newStationModel(conn, c, opts...), conn: conn}
}

const stationSoftFilter = "`deleted_at` is null"

func (m *customStationModel) InsertTx(ctx context.Context, session sqlx.Session, data *Station) error {
	query := "insert into `station` (`station_id`,`station_no`,`name`,`type`,`status`,`province`,`city`,`address`,`longitude`,`latitude`,`capacity_kwh`,`power_kw`,`grid_status`,`order_no`,`remark`,`tenant_id`,`created_by`,`updated_by`) values (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.StationId, data.StationNo, data.Name, data.Type, data.Status,
		data.Province, data.City, data.Address, data.Longitude, data.Latitude,
		data.CapacityKwh, data.PowerKw, data.GridStatus, data.OrderNo, data.Remark,
		data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customStationModel) FindOneScoped(ctx context.Context, tenantId, stationId int64) (*Station, error) {
	var res Station
	query := "select " + stationRows + " from `station` where `station_id` = ? and `tenant_id` = ? and " + stationSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, stationId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customStationModel) FindOneByOrderNoScoped(ctx context.Context, tenantId int64, orderNo string) (*Station, error) {
	var res Station
	query := "select " + stationRows + " from `station` where `tenant_id` = ? and `order_no` = ? and " + stationSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, tenantId, orderNo); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customStationModel) FindOneByNoScoped(ctx context.Context, tenantId int64, stationNo string) (*Station, error) {
	var res Station
	query := "select " + stationRows + " from `station` where `tenant_id` = ? and `station_no` = ? and " + stationSoftFilter
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, tenantId, stationNo); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customStationModel) ListPage(ctx context.Context, tenantId int64, keyword, status, stationType string, limit, offset int) ([]*Station, int64, error) {
	where := "`tenant_id` = ? and " + stationSoftFilter
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`station_no` like ? or `name` like ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if stationType != "" {
		where += " and `type` = ?"
		args = append(args, stationType)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `station` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Station
	args = append(args, limit, offset)
	query := "select " + stationRows + " from `station` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ---- station_device ----

var _ StationDeviceModel = (*customStationDeviceModel)(nil)

type (
	StationDeviceModel interface {
		stationDeviceModel
		// InsertOnDupTx 绑定（uk_station_device 冲突 = 重放，1062 调用方吞掉）。
		InsertOnDupTx(ctx context.Context, session sqlx.Session, data *StationDevice) error
		ListByStation(ctx context.Context, tenantId, stationId int64) ([]*StationDevice, error)
		// FindByDevice 设备反查场站（含场站 join 字段由 logic 组装）。
		FindOneByDevice(ctx context.Context, tenantId, deviceId int64) (*StationDevice, error)
		CountByStation(ctx context.Context, tenantId, stationId int64) (int64, error)
	}

	customStationDeviceModel struct {
		*defaultStationDeviceModel
		conn sqlx.SqlConn
	}
)

func NewStationDeviceModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationDeviceModel {
	return &customStationDeviceModel{defaultStationDeviceModel: newStationDeviceModel(conn, c, opts...), conn: conn}
}

func (m *customStationDeviceModel) InsertOnDupTx(ctx context.Context, session sqlx.Session, data *StationDevice) error {
	query := "insert into `station_device` (`id`,`station_id`,`device_id`,`sn`,`role`,`bound_at`,`bound_by`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.Id, data.StationId, data.DeviceId, data.Sn, data.Role,
		data.BoundAt, data.BoundBy, data.TenantId, data.CreatedBy)
	return err
}

func (m *customStationDeviceModel) ListByStation(ctx context.Context, tenantId, stationId int64) ([]*StationDevice, error) {
	var list []*StationDevice
	query := "select " + stationDeviceRows + " from `station_device` where `station_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `id`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, stationId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customStationDeviceModel) FindOneByDevice(ctx context.Context, tenantId, deviceId int64) (*StationDevice, error) {
	var res StationDevice
	query := "select " + stationDeviceRows + " from `station_device` where `device_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, deviceId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customStationDeviceModel) CountByStation(ctx context.Context, tenantId, stationId int64) (int64, error) {
	var n int64
	query := "select count(*) from `station_device` where `station_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.conn.QueryRowCtx(ctx, &n, query, stationId, tenantId); err != nil {
		return 0, err
	}
	return n, nil
}

// ---- station_staff ----

var _ StationStaffModel = (*customStationStaffModel)(nil)

type (
	StationStaffModel interface {
		stationStaffModel
		InsertOnDupTx(ctx context.Context, session sqlx.Session, data *StationStaff) error
		// SoftRemoveTx 逻辑移除（station_id+user_id）。
		SoftRemoveTx(ctx context.Context, session sqlx.Session, tenantId, stationId, userId int64) (int64, error)
		ListByStation(ctx context.Context, tenantId, stationId int64) ([]*StationStaff, error)
	}

	customStationStaffModel struct {
		*defaultStationStaffModel
		conn sqlx.SqlConn
	}
)

func NewStationStaffModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationStaffModel {
	return &customStationStaffModel{defaultStationStaffModel: newStationStaffModel(conn, c, opts...), conn: conn}
}

func (m *customStationStaffModel) InsertOnDupTx(ctx context.Context, session sqlx.Session, data *StationStaff) error {
	query := "insert into `station_staff` (`id`,`station_id`,`user_id`,`staff_type`,`shift`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.Id, data.StationId, data.UserId, data.StaffType, data.Shift, data.TenantId, data.CreatedBy)
	return err
}

func (m *customStationStaffModel) SoftRemoveTx(ctx context.Context, session sqlx.Session, tenantId, stationId, userId int64) (int64, error) {
	res, err := session.ExecCtx(ctx,
		"update `station_staff` set `deleted_at`=? where `station_id`=? and `user_id`=? and `tenant_id`=? and `deleted_at` is null",
		time.Now(), stationId, userId, tenantId)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (m *customStationStaffModel) ListByStation(ctx context.Context, tenantId, stationId int64) ([]*StationStaff, error) {
	var list []*StationStaff
	query := "select " + stationStaffRows + " from `station_staff` where `station_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `id`"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, stationId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

// ---- station_topology ----

var _ StationTopologyModel = (*customStationTopologyModel)(nil)

type (
	StationTopologyModel interface {
		stationTopologyModel
		InsertTx(ctx context.Context, session sqlx.Session, data *StationTopology) error
		// FindLatest 最新版本拓扑。
		FindLatest(ctx context.Context, tenantId, stationId int64) (*StationTopology, error)
		// NextVersion 版本号递增（logic 层事务内调用）。
		NextVersion(ctx context.Context, session sqlx.Session, stationId int64) (int64, error)
	}

	customStationTopologyModel struct {
		*defaultStationTopologyModel
		conn sqlx.SqlConn
	}
)

func NewStationTopologyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) StationTopologyModel {
	return &customStationTopologyModel{defaultStationTopologyModel: newStationTopologyModel(conn, c, opts...), conn: conn}
}

func (m *customStationTopologyModel) InsertTx(ctx context.Context, session sqlx.Session, data *StationTopology) error {
	query := "insert into `station_topology` (`id`,`station_id`,`version`,`nodes`,`edges`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.Id, data.StationId, data.Version, data.Nodes, data.Edges, data.TenantId, data.CreatedBy)
	return err
}

func (m *customStationTopologyModel) FindLatest(ctx context.Context, tenantId, stationId int64) (*StationTopology, error) {
	var res StationTopology
	query := "select " + stationTopologyRows + " from `station_topology` where `station_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `version` desc limit 1"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, stationId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customStationTopologyModel) NextVersion(ctx context.Context, session sqlx.Session, stationId int64) (int64, error) {
	var v int64
	query := "select coalesce(max(`version`),0) from `station_topology` where `station_id` = ? and `deleted_at` is null"
	if err := session.QueryRowCtx(ctx, &v, query, stationId); err != nil {
		return 0, err
	}
	return v + 1, nil
}
