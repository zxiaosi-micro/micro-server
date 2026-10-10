// 拓扑/监控/人员 logic（S6-02：SaveTopology/GetTopology/GetStationMonitor/AddStationStaff/RemoveStationStaff/ListStationStaff）。
package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"micro-server/services/station/internal/model"
	"micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"

	"github.com/zeromicro/go-zero/core/logx"
)

// SaveTopology 版本化保存（version 递增；React Flow 直喂数据形状，FR-STN-004）。
func (l *SaveTopologyLogic) SaveTopology(in *pb.SaveTopologyReq) (*pb.SaveTopologyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	var nodes, edges any
	if err := json.Unmarshal([]byte(in.NodesJson), &nodes); err != nil {
		return nil, errTopologyBadJson
	}
	if err := json.Unmarshal([]byte(in.EdgesJson), &edges); err != nil {
		return nil, errTopologyBadJson
	}
	if _, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, in.StationId); err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	var version int64
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		v, err := l.svcCtx.Models.Topology.NextVersion(ctx, session, in.StationId)
		if err != nil {
			return err
		}
		version = v
		return l.svcCtx.Models.Topology.InsertTx(ctx, session, &model.StationTopology{
			Id:        l.svcCtx.Snowflake.MustNextID(),
			StationId: in.StationId,
			Version:   v,
			Nodes:     in.NodesJson,
			Edges:     in.EdgesJson,
			TenantId:  tid,
			CreatedBy: toNullInt64(ctxkit.UID(l.ctx)),
		})
	})
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("topology saved station_id=%d version=%d", in.StationId, version)
	return &pb.SaveTopologyResp{Version: version}, nil
}

// GetTopology 最新版拓扑。
func (l *GetTopologyLogic) GetTopology(in *pb.GetTopologyReq) (*pb.GetTopologyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	t, err := l.svcCtx.Models.Topology.FindLatest(l.ctx, tid, in.StationId)
	if err != nil {
		if err == model.ErrNotFound {
			return &pb.GetTopologyResp{Topology: &pb.TopologyView{Version: 0, NodesJson: "[]", EdgesJson: "[]"}}, nil
		}
		return nil, err
	}
	return &pb.GetTopologyResp{Topology: &pb.TopologyView{
		Version:   int64(t.Version),
		NodesJson: string(t.Nodes),
		EdgesJson: string(t.Edges),
		UpdatedAt: t.UpdatedAt.UnixMilli(),
	}}, nil
}

// GetStationMonitor 聚合监控（影子只读；功率合计/SOC 均值/在线数，FR-STN-005）。
func (l *GetStationMonitorLogic) GetStationMonitor(in *pb.GetStationMonitorReq) (*pb.GetStationMonitorResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, in.StationId); err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	devices, err := l.svcCtx.Models.Device.ListByStation(l.ctx, tid, in.StationId)
	if err != nil {
		return nil, err
	}
	staleSec := l.svcCtx.Config.MonitorStaleSec
	if cc := confcenterCurrent().MonitorStaleSec; cc > 0 {
		staleSec = cc
	}
	staleMs := int64(staleSec) * 1000

	resp := &pb.GetStationMonitorResp{
		StationId:   in.StationId,
		DeviceCount: int32(len(devices)),
		Items:       []*pb.StationMonitorItem{},
	}
	var socSum float64
	socN := 0
	now := time.Now().UnixMilli()
	for _, d := range devices {
		item := &pb.StationMonitorItem{DeviceId: d.DeviceId, Sn: d.Sn}
		if l.svcCtx.ShadowRd != nil {
			if vals, gerr := l.svcCtx.ShadowRd.HgetallCtx(l.ctx, "micro:iot:shadow:"+d.Sn); gerr == nil && len(vals) > 0 {
				ts, _ := strconv.ParseInt(vals["ts"], 10, 64)
				item.Ts = ts
				item.Online = ts > 0 && now-ts <= staleMs
				item.Soc = parseFloat(vals["soc"])
				item.Power = parseFloat(vals["power"])
				item.Voltage = parseFloat(vals["voltage"])
				item.Temperature = parseFloat(vals["temperature"])
				if item.Soc > 0 {
					socSum += item.Soc
					socN++
				}
				resp.TotalPower += item.Power
			}
		}
		if item.Online {
			resp.OnlineCount++
		}
		resp.Items = append(resp.Items, item)
	}
	if socN > 0 {
		resp.AvgSoc = socSum / float64(socN)
	}
	return resp, nil
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// AddStationStaff 加驻场人员（FR-STN-006；data_scope 锁定由 identity/authz 侧承载）。
func (l *AddStationStaffLogic) AddStationStaff(in *pb.AddStationStaffReq) (*pb.AddStationStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 {
		return nil, errStaffNotFound
	}
	switch in.StaffType {
	case "RESIDENT", "INSPECTOR", "MANAGER", "":
		in.StaffType = orDefault(in.StaffType, "RESIDENT")
	default:
		return nil, errStaffTypeBad
	}
	if _, err := l.svcCtx.Models.Station.FindOneScoped(l.ctx, tid, in.StationId); err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, err
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return l.svcCtx.Models.Staff.InsertOnDupTx(ctx, session, &model.StationStaff{
			Id:        l.svcCtx.Snowflake.MustNextID(),
			StationId: in.StationId,
			UserId:    in.UserId,
			StaffType: in.StaffType,
			Shift:     toNullString(in.Shift),
			TenantId:  tid,
			CreatedBy: toNullInt64(ctxkit.UID(l.ctx)),
		})
	})
	if err != nil && !isDupKey(err) {
		return nil, err
	}
	return &pb.AddStationStaffResp{}, nil
}

// RemoveStationStaff 移除驻场人员。
func (l *RemoveStationStaffLogic) RemoveStationStaff(in *pb.RemoveStationStaffReq) (*pb.RemoveStationStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		_, err := l.svcCtx.Models.Staff.SoftRemoveTx(ctx, session, tid, in.StationId, in.UserId)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &pb.RemoveStationStaffResp{}, nil
}

// ListStationStaff 人员清单。
func (l *ListStationStaffLogic) ListStationStaff(in *pb.ListStationStaffReq) (*pb.ListStationStaffResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Staff.ListByStation(l.ctx, tid, in.StationId)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListStationStaffResp{List: []*pb.StationStaffView{}}
	for _, s := range list {
		v := &pb.StationStaffView{
			Id: s.Id, StationId: s.StationId, UserId: s.UserId,
			StaffType: s.StaffType, CreatedAt: s.CreatedAt.UnixMilli(),
		}
		if s.Shift.Valid {
			v.Shift = s.Shift.String
		}
		resp.List = append(resp.List, v)
	}
	return resp, nil
}
