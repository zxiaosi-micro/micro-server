// 影子/拓扑/指令 logic（S6-01：GetDeviceShadow/GetDeviceTopology/SaveDeviceTopology/SendCommand/ListCmd）。
package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/internal/shadow"
	"micro-server/services/device/internal/svc"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var cmdTypeAllow = map[string]bool{
	"REBOOT": true, "RELAY_SET": true, "QUERY": true, "OTA_UPGRADE": true,
	"SET_PARAM": true, "PING": true,
}

// GetDeviceShadow 最近遥测影子（Redis；iotingest writer 写入，弱网可查 FR-IOT-005）。
func (l *GetDeviceShadowLogic) GetDeviceShadow(in *pb.GetDeviceShadowReq) (*pb.GetDeviceShadowResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Sn == "" {
		return nil, errSnRequired
	}
	if _, err := l.svcCtx.Models.Device.FindOneBySnScoped(l.ctx, tid, in.Sn); err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	if l.svcCtx.Shadow == nil {
		return nil, errShadowNotFound
	}
	snap, err := l.svcCtx.Shadow.Get(l.ctx, in.Sn)
	if err != nil {
		if err == shadow.ErrNotFound {
			return nil, errShadowNotFound
		}
		return nil, err
	}
	resp := &pb.GetDeviceShadowResp{Sn: snap.Sn, Ts: snap.Ts, Raw: snap.Raw, Metrics: []*pb.ShadowMetric{}}
	for _, k := range []string{"soc", "voltage", "current", "temperature", "power"} {
		if v, ok := snap.Metrics[k]; ok {
			f, _ := strconv.ParseFloat(v, 64)
			resp.Metrics = append(resp.Metrics, &pb.ShadowMetric{Key: k, Value: f})
		}
	}
	return resp, nil
}

// GetDeviceTopology 设备拓扑树（FR-DEV-004）。
func (l *GetDeviceTopologyLogic) GetDeviceTopology(in *pb.GetDeviceTopologyReq) (*pb.GetDeviceTopologyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId); err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	nodes, err := l.svcCtx.Models.Topology.ListByDevice(l.ctx, tid, in.DeviceId)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetDeviceTopologyResp{Nodes: []*pb.TopologyNode{}}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, &pb.TopologyNode{
			NodeId: n.NodeId, ParentId: n.ParentId.Int64, NodeType: n.NodeType,
			NodeName: n.NodeName, Sort: int32(n.Sort),
		})
	}
	return resp, nil
}

// SaveDeviceTopology 全量替换拓扑（tx 内软删 + 重插）。
func (l *SaveDeviceTopologyLogic) SaveDeviceTopology(in *pb.SaveDeviceTopologyReq) (*pb.SaveDeviceTopologyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId); err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	// 层级校验（电芯→模组→电池包→充电桩，最多 4 级）
	for _, n := range in.Nodes {
		switch n.NodeType {
		case "CELL", "MODULE", "PACK", "CHARGER":
		default:
			return nil, errTopologyTypeBad
		}
	}
	// parent_ref 引用校验 + 深度计算
	depth := map[int64]int{}
	for i, n := range in.Nodes {
		ref := int64(i) // 本节点临时序号
		if n.ParentRef == 0 {
			depth[ref] = 1
			continue
		}
		if n.ParentRef < 0 || n.ParentRef >= int64(len(in.Nodes)) || n.ParentRef >= ref {
			return nil, errTopologyTypeBad
		}
		depth[ref] = depth[n.ParentRef] + 1
		if depth[ref] > 4 {
			return nil, errTopologyTooDeep
		}
	}
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := l.svcCtx.Models.Topology.SoftDeleteByDeviceTx(ctx, session, tid, in.DeviceId); err != nil {
			return err
		}
		ids := make([]int64, len(in.Nodes))
		for i := range in.Nodes {
			ids[i] = l.svcCtx.Snowflake.MustNextID()
		}
		for i, n := range in.Nodes {
			var parentId sql.NullInt64
			if n.ParentRef != 0 {
				parentId = sql.NullInt64{Int64: ids[n.ParentRef], Valid: true}
			}
			if err := l.svcCtx.Models.Topology.InsertTx(ctx, session, &model.DeviceTopology{
				NodeId:   ids[i],
				DeviceId: in.DeviceId,
				ParentId: parentId,
				NodeType: n.NodeType,
				NodeName: n.NodeName,
				Sort:     int64(n.Sort),
				TenantId: tid,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &pb.SaveDeviceTopologyResp{}, nil
}

// SendCommand 指令下发（FR-IOT-006：cmd 表状态机 + MQTT QoS1 + TimingWheel 3s 计 ACK；
// 重试 ≤3 由 cron 扫描兜底；审计 cmd_audit 独立流 100% 留痕）。
func (l *SendCommandLogic) SendCommand(in *pb.SendCommandReq) (*pb.SendCommandResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if !cmdTypeAllow[in.CmdType] {
		return nil, errCmdTypeBad
	}
	if in.ParamsJson != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(in.ParamsJson), &raw); err != nil {
			return nil, errParamsBadJson
		}
	}
	// 调用方幂等键重放
	if in.ClientCmdId != "" {
		if exist, err := l.svcCtx.Models.Cmd.FindOneByClientCmdId(l.ctx, tid, in.ClientCmdId); err == nil {
			return &pb.SendCommandResp{CmdId: exist.CmdId}, nil
		} else if err != model.ErrNotFound {
			return nil, err
		}
	}
	var d *model.Device
	if in.DeviceId > 0 {
		d, err = l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId)
	} else if in.Sn != "" {
		d, err = l.svcCtx.Models.Device.FindOneBySnScoped(l.ctx, tid, in.Sn)
	} else {
		return nil, errDeviceIdOrSn
	}
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	op := opUID(l.ctx)
	timeoutMs := l.svcCtx.Config.CmdAckTimeoutMs
	if cc := confcenterAckTimeout(); cc > 0 {
		timeoutMs = cc
	}
	cmdId := l.svcCtx.Snowflake.MustNextID()
	params := toNullString(in.ParamsJson)

	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return l.svcCtx.Models.Cmd.InsertTx(ctx, session, &model.Cmd{
			CmdId:       cmdId,
			DeviceId:    d.DeviceId,
			Sn:          d.Sn,
			CmdType:     in.CmdType,
			Params:      params,
			Status:      "PENDING",
			MaxRetry:    3,
			NextExecAt:  time.Now(),
			ClientCmdId: toNullString(in.ClientCmdId),
			Operator:    toNullInt64(op),
			TenantId:    tid,
		})
	})
	if err != nil {
		return nil, err
	}
	// 指令审计（下发动作即留痕，FR-IOT-006 100% 审计）
	writeCmdAudit(l.ctx, l.svcCtx, cmdId, d.Sn, in.CmdType, in.ParamsJson, "DISPATCHED", op, tid)

	// 立即投递 + 挂 ACK 超时轮（失败不返回错误——cron 兜底重试，ADR-09）
	if err := dispatchCmd(l.ctx, l.svcCtx, tid, cmdId, d.Sn, in.CmdType, in.ParamsJson); err != nil {
		logx.WithContext(l.ctx).Errorf("cmd 首投失败（cron 兜底重试）cmd_id=%d: %v", cmdId, err)
	} else {
		watchAck(l.svcCtx, tid, cmdId, time.Duration(timeoutMs)*time.Millisecond)
	}
	return &pb.SendCommandResp{CmdId: cmdId}, nil
}

// ListCmd 指令列表。
func (l *ListCmdLogic) ListCmd(in *pb.ListCmdReq) (*pb.ListCmdResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Cmd.ListPage(l.ctx, tid, in.DeviceId, in.Sn, in.Status, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListCmdResp{Total: total, List: []*pb.CmdView{}}
	for _, c := range list {
		v := &pb.CmdView{
			CmdId: c.CmdId, DeviceId: c.DeviceId, Sn: c.Sn, CmdType: c.CmdType,
			Status: c.Status, RetryCount: int32(c.RetryCount), Operator: c.Operator.Int64,
			CreatedAt: c.CreatedAt.UnixMilli(),
		}
		if c.Params.Valid {
			v.ParamsJson = c.Params.String
		}
		if c.AckedAt.Valid {
			v.AckedAt = c.AckedAt.Time.UnixMilli()
		}
		if c.FailReason.Valid {
			v.FailReason = c.FailReason.String
		}
		resp.List = append(resp.List, v)
	}
	return resp, nil
}

// dispatchCmd MQTT 投递 + cmd PENDING→SENT（投递失败保持 PENDING 等 cron）。
func dispatchCmd(ctx context.Context, sc *svc.ServiceContext, tid, cmdId int64, sn, cmdType, params string) error {
	payload := map[string]any{
		"cmd_id": fmt.Sprintf("%d", cmdId), "type": cmdType, "ts": time.Now().UnixMilli(),
	}
	if params != "" {
		var p any
		if err := json.Unmarshal([]byte(params), &p); err == nil {
			payload["params"] = p
		}
	}
	raw, _ := json.Marshal(payload)
	if err := sc.Emqx.PublishDown(ctx, sn, string(raw)); err != nil {
		return err
	}
	return sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		_, err := sc.Models.Cmd.CasStatusTx(ctx, session, tid, cmdId, "PENDING", "SENT", "")
		return err
	})
}

// watchAck 挂 TimingWheel ACK 计时（内存态加速；进程重启由 cron 扫描兜底，02 §9.6）。
func watchAck(sc *svc.ServiceContext, tid, cmdId int64, timeout time.Duration) {
	key := fmt.Sprintf("%d:%d", tid, cmdId)
	// 已挂同键计时器则续期（MoveTimer：指令重投场景）
	if err := sc.AckWheel.MoveTimer(key, timeout); err == nil {
		return
	}
	if err := sc.AckWheel.SetTimer(key, func() {
		// 3s 无 ACK → 置回 PENDING + next_exec_at 退避（cron 扫描重试）
		ctx := tenantxWith(context.Background(), tid)
		err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			return sc.Models.Cmd.IncRetryTx(ctx, session, cmdId, time.Now())
		})
		if err != nil {
			logx.Errorf("cmd ACK 超时回退失败（cron 兜底）cmd_id=%d: %v", cmdId, err)
		}
	}, timeout); err != nil {
		logx.Errorf("ACK 计时挂载失败 cmd_id=%d: %v", cmdId, err)
	}
}

// confcenterAckTimeout configcenter 覆盖值（热调）。
func confcenterAckTimeout() int64 {
	return confcenterCurrent().CmdAckTimeoutMs
}

var _ = collection.NewTimingWheel
var _ = fmt.Sprintf
