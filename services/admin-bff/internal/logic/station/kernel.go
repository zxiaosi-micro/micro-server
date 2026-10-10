// station 域 BFF logic 实现（S6-02；goctl 骨架方法体填充；转发 station RPC + string↔int64 归一，E8）。

package station

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	spb "micro-server/services/station/pb"
)

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func i64s(v int64) string { return strconv.FormatInt(v, 10) }

func stationView(s *spb.StationView) types.StationView {
	return types.StationView{
		StationId: i64s(s.StationId), StationNo: s.StationNo, Name: s.Name,
		Type: s.Type, Status: s.Status, Province: s.Province, City: s.City,
		Address: s.Address, Longitude: s.Longitude, Latitude: s.Latitude,
		CapacityKwh: s.CapacityKwh, PowerKw: s.PowerKw, GridStatus: s.GridStatus,
		OrderNo: s.OrderNo, CreatedAt: s.CreatedAt,
	}
}

// ListStations 场站列表。
func (l *ListStationsLogic) ListStations(req *types.StationListReq) (*types.StationListResp, error) {
	out, err := l.svcCtx.Station.ListStation(l.ctx, &spb.ListStationReq{
		Keyword: req.Keyword, Status: req.Status, Type: req.Type,
		Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.StationListResp{Total: int(out.Total), List: []types.StationView{}}
	for _, s := range out.List {
		resp.List = append(resp.List, stationView(s))
	}
	return resp, nil
}

// GetStation 场站详情。
func (l *GetStationLogic) GetStation(req *types.StationNoPath) (*types.StationGetResp, error) {
	out, err := l.svcCtx.Station.GetStation(l.ctx, &spb.GetStationReq{StationId: parseI64(req.StationId)})
	if err != nil {
		return nil, err
	}
	return &types.StationGetResp{Station: stationView(out.Station)}, nil
}

// CreateStation 场站创建（Saga 步骤6 手工兜底）。
func (l *CreateStationLogic) CreateStation(req *types.StationCreateReq) (*types.StationCreateResp, error) {
	devices := make([]*spb.StationDeviceBind, 0, len(req.Devices))
	for _, d := range req.Devices {
		devices = append(devices, &spb.StationDeviceBind{DeviceId: parseI64(d.DeviceId), Sn: d.Sn, Role: d.Role})
	}
	out, err := l.svcCtx.Station.CreateStation(l.ctx, &spb.CreateStationReq{
		OrderNo: req.OrderNo, StationNo: req.StationNo, Name: req.Name, Type: req.Type,
		Province: req.Province, City: req.City, Address: req.Address,
		Longitude: req.Longitude, Latitude: req.Latitude,
		CapacityKwh: req.CapacityKwh, PowerKw: req.PowerKw, GridStatus: req.GridStatus,
		Devices: devices,
	})
	if err != nil {
		return nil, err
	}
	return &types.StationCreateResp{StationId: i64s(out.StationId)}, nil
}

// BindStationDevices 场站设备绑定。
func (l *BindStationDevicesLogic) BindStationDevices(req *types.StationBindReq) (*types.StationBindResp, error) {
	devices := make([]*spb.StationDeviceBind, 0, len(req.Devices))
	for _, d := range req.Devices {
		devices = append(devices, &spb.StationDeviceBind{DeviceId: parseI64(d.DeviceId), Sn: d.Sn, Role: d.Role})
	}
	out, err := l.svcCtx.Station.BindDevices(l.ctx, &spb.BindDevicesReq{
		StationId: parseI64(req.StationId), Devices: devices, BoundBy: req.BoundBy,
	})
	if err != nil {
		return nil, err
	}
	return &types.StationBindResp{Bound: int(out.Bound)}, nil
}

// ListStationDevices 场站设备清单。
func (l *ListStationDevicesLogic) ListStationDevices(req *types.StationNoPath) (*types.StationDeviceListResp, error) {
	out, err := l.svcCtx.Station.ListStationDevices(l.ctx, &spb.ListStationDevicesReq{StationId: parseI64(req.StationId)})
	if err != nil {
		return nil, err
	}
	resp := &types.StationDeviceListResp{List: []types.StationDeviceView{}}
	for _, d := range out.List {
		resp.List = append(resp.List, types.StationDeviceView{
			Id: i64s(d.Id), StationId: i64s(d.StationId), DeviceId: i64s(d.DeviceId),
			Sn: d.Sn, Role: d.Role, BoundAt: d.BoundAt,
		})
	}
	return resp, nil
}

// GetStationTopology 拓扑查询。
func (l *GetStationTopologyLogic) GetStationTopology(req *types.StationNoPath) (*types.StationTopologyResp, error) {
	out, err := l.svcCtx.Station.GetTopology(l.ctx, &spb.GetTopologyReq{StationId: parseI64(req.StationId)})
	if err != nil {
		return nil, err
	}
	return &types.StationTopologyResp{Topology: types.StationTopologyView{
		Version: int(out.Topology.Version), NodesJson: out.Topology.NodesJson,
		EdgesJson: out.Topology.EdgesJson, UpdatedAt: out.Topology.UpdatedAt,
	}}, nil
}

// SaveStationTopology 拓扑保存（版本化）。
func (l *SaveStationTopologyLogic) SaveStationTopology(req *types.StationTopologySaveReq) (*types.StationTopologySaveResp, error) {
	out, err := l.svcCtx.Station.SaveTopology(l.ctx, &spb.SaveTopologyReq{
		StationId: parseI64(req.StationId), NodesJson: req.NodesJson, EdgesJson: req.EdgesJson,
	})
	if err != nil {
		return nil, err
	}
	return &types.StationTopologySaveResp{Version: int(out.Version)}, nil
}

// GetStationMonitor 聚合监控（影子只读）。
func (l *GetStationMonitorLogic) GetStationMonitor(req *types.StationNoPath) (*types.StationMonitorResp, error) {
	out, err := l.svcCtx.Station.GetStationMonitor(l.ctx, &spb.GetStationMonitorReq{StationId: parseI64(req.StationId)})
	if err != nil {
		return nil, err
	}
	resp := &types.StationMonitorResp{
		StationId: i64s(out.StationId), DeviceCount: int(out.DeviceCount),
		OnlineCount: int(out.OnlineCount), AvgSoc: out.AvgSoc, TotalPower: out.TotalPower,
		Items: []types.StationMonitorItem{},
	}
	for _, it := range out.Items {
		resp.Items = append(resp.Items, types.StationMonitorItem{
			DeviceId: i64s(it.DeviceId), Sn: it.Sn, Online: it.Online,
			Soc: it.Soc, Power: it.Power, Voltage: it.Voltage, Temperature: it.Temperature, Ts: it.Ts,
		})
	}
	return resp, nil
}

// ListStationStaff 驻场人员。
func (l *ListStationStaffLogic) ListStationStaff(req *types.StationNoPath) (*types.StationStaffListResp, error) {
	out, err := l.svcCtx.Station.ListStationStaff(l.ctx, &spb.ListStationStaffReq{StationId: parseI64(req.StationId)})
	if err != nil {
		return nil, err
	}
	resp := &types.StationStaffListResp{List: []types.StationStaffView{}}
	for _, s := range out.List {
		resp.List = append(resp.List, types.StationStaffView{
			Id: i64s(s.Id), StationId: i64s(s.StationId), UserId: i64s(s.UserId),
			StaffType: s.StaffType, Shift: s.Shift, CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}

// AddStationStaff 添加驻场人员。
func (l *AddStationStaffLogic) AddStationStaff(req *types.StationStaffAddReq) (*types.SimpleResp, error) {
	_, err := l.svcCtx.Station.AddStationStaff(l.ctx, &spb.AddStationStaffReq{
		StationId: parseI64(req.StationId), UserId: parseI64(req.UserId),
		StaffType: req.StaffType, Shift: req.Shift,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// RemoveStationStaff 移除驻场人员。
func (l *RemoveStationStaffLogic) RemoveStationStaff(req *types.StationStaffRemoveReq) (*types.SimpleResp, error) {
	_, err := l.svcCtx.Station.RemoveStationStaff(l.ctx, &spb.RemoveStationStaffReq{
		StationId: parseI64(req.StationId), UserId: parseI64(req.UserId),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// GetDeviceStation 设备反查场站。
func (l *GetDeviceStationLogic) GetDeviceStation(req *types.DeviceNoPath) (*types.DeviceStationResp, error) {
	out, err := l.svcCtx.Station.GetDeviceStation(l.ctx, &spb.GetDeviceStationReq{DeviceId: parseI64(req.DeviceId)})
	if err != nil {
		return nil, err
	}
	return &types.DeviceStationResp{
		StationId: i64s(out.Station.StationId), StationNo: out.Station.StationNo,
		StationName: out.Station.StationName, Role: out.Station.Role,
	}, nil
}

// ---- Logic 结构体（goctl 骨架等价物；重新生成 goctl 时删除 *_logic.go 保留本文件）----

type ListStationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListStationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationsLogic {
	return &ListStationsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetStationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationLogic {
	return &GetStationLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateStationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStationLogic {
	return &CreateStationLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type BindStationDevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindStationDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindStationDevicesLogic {
	return &BindStationDevicesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListStationDevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListStationDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationDevicesLogic {
	return &ListStationDevicesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetStationTopologyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStationTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationTopologyLogic {
	return &GetStationTopologyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SaveStationTopologyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveStationTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveStationTopologyLogic {
	return &SaveStationTopologyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetStationMonitorLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStationMonitorLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationMonitorLogic {
	return &GetStationMonitorLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListStationStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationStaffLogic {
	return &ListStationStaffLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type AddStationStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddStationStaffLogic {
	return &AddStationStaffLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type RemoveStationStaffLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRemoveStationStaffLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RemoveStationStaffLogic {
	return &RemoveStationStaffLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetDeviceStationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDeviceStationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceStationLogic {
	return &GetDeviceStationLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
