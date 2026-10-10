// 场站主数据 + 绑定 logic（S6-02：CreateStation/GetStation/ListStation/BindDevices/ListStationDevices/GetDeviceStation）。
package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"micro-server/services/station/internal/model"
	"micro-server/services/station/internal/svc"
	"micro-server/services/station/pb"

	devpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// CreateStation 建站 + 绑定清单同事务（Saga 步骤 6 对端；幂等键 tenant_id+order_no——
// 同订单重放返回已有场站，FR-STN-003）。
func (l *CreateStationLogic) CreateStation(in *pb.CreateStationReq) (*pb.CreateStationResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" {
		return nil, errNameRequired
	}
	switch in.Type {
	case "ESS", "CHARGING", "HESS", "":
		in.Type = orDefault(in.Type, "ESS")
	default:
		return nil, errTypeBad
	}
	// Saga 幂等重放：同 order_no 已建站 → 直接返回
	if in.OrderNo != "" {
		if exist, err := l.svcCtx.Models.Station.FindOneByOrderNoScoped(l.ctx, tid, in.OrderNo); err == nil {
			logx.WithContext(l.ctx).Infof("station 幂等重放命中 order_no=%s station_id=%d", in.OrderNo, exist.StationId)
			return &pb.CreateStationResp{StationId: exist.StationId}, nil
		} else if err != model.ErrNotFound {
			return nil, err
		}
	}
	op := ctxkit.UID(l.ctx)
	stationId := l.svcCtx.Snowflake.MustNextID()
	stationNo := in.StationNo
	if stationNo == "" {
		stationNo = fmt.Sprintf("STN%d", stationId)
	}

	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		now := time.Now()
		if err := l.svcCtx.Models.Station.InsertTx(ctx, session, &model.Station{
			StationId:   stationId,
			StationNo:   stationNo,
			Name:        in.Name,
			Type:        in.Type,
			Status:      "ACTIVE",
			Province:    in.Province,
			City:        in.City,
			Address:     in.Address,
			Longitude:   nullFloat(in.Longitude),
			Latitude:    nullFloat(in.Latitude),
			CapacityKwh: nullFloat(in.CapacityKwh),
			PowerKw:     nullFloat(in.PowerKw),
			GridStatus:  toNullString(in.GridStatus),
			OrderNo:     toNullString(in.OrderNo),
			TenantId:    tid,
			CreatedBy:   toNullInt64(op),
			UpdatedBy:   toNullInt64(op),
		}); err != nil {
			return err
		}
		// 绑定清单同事务（FR-STN-003 建站+绑清单原子）
		return bindDevicesInTx(ctx, session, l.svcCtx, tid, stationId, in.Devices, orDefault(in.OrderNo, "manual"), op, now)
	})
	if err != nil {
		if isDupKey(err) {
			// station_no 冲突或并发重放：查回已有场站
			if exist, gerr := l.svcCtx.Models.Station.FindOneByNoScoped(l.ctx, tid, stationNo); gerr == nil {
				return &pb.CreateStationResp{StationId: exist.StationId}, nil
			}
			return nil, errStationNoUsed
		}
		return nil, err
	}

	// station_created 事件（ops S7 消费建巡检计划；事务外独立 Emit——建站已提交）
	if err := eventbus.Emit(l.ctx, l.svcCtx.Conn, stationCreatedEvent(tid, stationId, stationNo, in.Name, in.OrderNo)); err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("station created no=%s name=%s order=%s devices=%d",
		stationNo, in.Name, in.OrderNo, len(in.Devices))
	return &pb.CreateStationResp{StationId: stationId}, nil
}

// bindDevicesInTx 绑定清单事务体（CreateStation/BindDevices 共用）。
func bindDevicesInTx(ctx context.Context, session sqlx.Session, sc *svc.ServiceContext, tid, stationId int64,
	devices []*pb.StationDeviceBind, boundBy string, op int64, now time.Time) error {
	for _, b := range devices {
		deviceId := b.DeviceId
		sn := b.Sn
		if deviceId == 0 && sn == "" {
			return errBindNoDevice
		}
		// sn 归一 device_id（经 device RPC；CreateStation Saga 传 SN 清单）
		if deviceId == 0 && sc.Device != nil {
			resp, err := sc.Device.GetDeviceBySn(ctx, &devpb.GetDeviceBySnReq{Sn: sn})
			if err != nil || resp.Device == nil {
				logx.WithContext(ctx).Errorf("绑定设备解析失败 sn=%s: %v（跳过）", sn, err)
				continue
			}
			deviceId = resp.Device.DeviceId
		}
		if deviceId == 0 {
			continue
		}
		if sn == "" {
			sn = fmt.Sprintf("dev:%d", deviceId)
		}
		role := orDefault(b.Role, "PACK")
		if err := sc.Models.Device.InsertOnDupTx(ctx, session, &model.StationDevice{
			Id:        sc.Snowflake.MustNextID(),
			StationId: stationId,
			DeviceId:  deviceId,
			Sn:        sn,
			Role:      role,
			BoundAt:   sql.NullTime{Time: now, Valid: true},
			BoundBy:   toNullString(boundBy),
			TenantId:  tid,
			CreatedBy: toNullInt64(op),
		}); err != nil {
			if isDupKey(err) {
				continue // 重复绑定幂等跳过
			}
			return err
		}
	}
	return nil
}

// GetStation 场站详情。
func (l *GetStationLogic) GetStation(in *pb.GetStationReq) (*pb.GetStationResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	s, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, in.StationId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	return &pb.GetStationResp{Station: stationView(s)}, nil
}

// ListStation 场站列表。
func (l *ListStationLogic) ListStation(in *pb.ListStationReq) (*pb.ListStationResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	if in.Status != "" && !isStationStatus(in.Status) {
		return nil, errStatusBad
	}
	list, total, err := l.svcCtx.Models.Station.ListPage(l.ctx, tid, in.Keyword, in.Status, in.Type, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListStationResp{Total: total, List: []*pb.StationView{}}
	for _, s := range list {
		resp.List = append(resp.List, stationView(s))
	}
	return resp, nil
}

func isStationStatus(s string) bool {
	switch s {
	case "ACTIVE", "SUSPENDED", "RETIRED":
		return true
	}
	return false
}

// BindDevices 追加绑定（手工补绑；bound_by=manual）。
func (l *BindDevicesLogic) BindDevices(in *pb.BindDevicesReq) (*pb.BindDevicesResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if len(in.Devices) == 0 {
		return nil, errBindEmpty
	}
	if _, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, in.StationId); err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	op := ctxkit.UID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return bindDevicesInTx(ctx, session, l.svcCtx, tid, in.StationId, in.Devices, orDefault(in.BoundBy, "manual"), op, time.Now())
	})
	if err != nil {
		return nil, err
	}
	return &pb.BindDevicesResp{Bound: int64(len(in.Devices))}, nil
}

// ListStationDevices 场站设备清单。
func (l *ListStationDevicesLogic) ListStationDevices(in *pb.ListStationDevicesReq) (*pb.ListStationDevicesResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Device.ListByStation(l.ctx, tid, in.StationId)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListStationDevicesResp{List: []*pb.StationDeviceView{}}
	for _, d := range list {
		v := &pb.StationDeviceView{Id: d.Id, StationId: d.StationId, DeviceId: d.DeviceId, Sn: d.Sn, Role: d.Role}
		if d.BoundAt.Valid {
			v.BoundAt = d.BoundAt.Time.UnixMilli()
		}
		resp.List = append(resp.List, v)
	}
	return resp, nil
}

// GetDeviceStation 设备反查场站（FR-STN-002）。
func (l *GetDeviceStationLogic) GetDeviceStation(in *pb.GetDeviceStationReq) (*pb.GetDeviceStationResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	deviceId := in.DeviceId
	if deviceId == 0 && in.Sn != "" {
		if l.svcCtx.Device == nil {
			return nil, errDeviceRpcMissing
		}
		resp, err := l.svcCtx.Device.GetDeviceBySn(l.ctx, &devpb.GetDeviceBySnReq{Sn: in.Sn})
		if err != nil || resp.Device == nil {
			return nil, errDeviceNotFound
		}
		deviceId = resp.Device.DeviceId
	}
	bind, err := l.svcCtx.Models.Device.FindOneByDevice(l.ctx, tid, deviceId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	s, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, bind.StationId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	return &pb.GetDeviceStationResp{Station: &pb.DeviceStationView{
		StationId:   s.StationId,
		StationNo:   s.StationNo,
		StationName: s.Name,
		Role:        bind.Role,
	}}, nil
}

// ---- 视图/事件/助手 ----

func stationView(s *model.Station) *pb.StationView {
	v := &pb.StationView{
		StationId: s.StationId, StationNo: s.StationNo, Name: s.Name,
		Type: s.Type, Status: s.Status, Province: s.Province, City: s.City, Address: s.Address,
		CreatedAt: s.CreatedAt.UnixMilli(),
	}
	if s.Longitude.Valid {
		v.Longitude = s.Longitude.Float64
	}
	if s.Latitude.Valid {
		v.Latitude = s.Latitude.Float64
	}
	if s.CapacityKwh.Valid {
		v.CapacityKwh = s.CapacityKwh.Float64
	}
	if s.PowerKw.Valid {
		v.PowerKw = s.PowerKw.Float64
	}
	if s.GridStatus.Valid {
		v.GridStatus = s.GridStatus.String
	}
	if s.OrderNo.Valid {
		v.OrderNo = s.OrderNo.String
	}
	return v
}

// stationCreatedEvent station.created 事件（ops S7 消费建巡检计划）。
func stationCreatedEvent(tid, stationId int64, stationNo, name, orderNo string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic:    eventbus.TopicStationCreated,
		Type:     eventbus.TypeStationCreated,
		Key:      stationNo,
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "station_id": stationId, "station_no": stationNo,
			"name": name, "order_no": orderNo,
		},
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func toNullInt64(v int64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}

func nullFloat(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: v != 0}
}
