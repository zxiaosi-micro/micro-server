// device 域 BFF logic 实现（S6-01；goctl 骨架方法体填充；转发 device RPC + string↔int64 归一，E8）。

package device

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"
)

func parseI64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func i64s(v int64) string { return strconv.FormatInt(v, 10) }

// ---- 设备 ----

func deviceView(d *dpb.DeviceView) types.DeviceView {
	return types.DeviceView{
		DeviceId: i64s(d.DeviceId), Sn: d.Sn, ProductKey: d.ProductKey,
		Model: d.Model, BatchNo: d.BatchNo, Status: d.Status,
		PartyId: i64s(d.PartyId), OrderNo: d.OrderNo, ActivatedAt: d.ActivatedAt,
		HasSecret: d.HasSecret, CreatedAt: d.CreatedAt,
	}
}

// ImportDevices 设备导入（一机一密自动开通；secrets 明文产线 CSV 一次）。
func (l *ImportDevicesLogic) ImportDevices(req *types.DeviceImportReq) (*types.DeviceImportResp, error) {
	items := make([]*dpb.ImportDeviceItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &dpb.ImportDeviceItem{Sn: it.Sn, ProductKey: it.ProductKey, Model: it.Model, BatchNo: it.BatchNo})
	}
	out, err := l.svcCtx.Device.ImportSN(l.ctx, &dpb.ImportSNReq{Items: items, Provision: req.Provision})
	if err != nil {
		return nil, err
	}
	resp := &types.DeviceImportResp{Imported: int(out.Imported), Secrets: []types.DeviceSecretRow{}, ProvisionErrors: out.ProvisionErrors}
	for _, s := range out.Secrets {
		resp.Secrets = append(resp.Secrets, types.DeviceSecretRow{Sn: s.Sn, Secret: s.Secret})
	}
	return resp, nil
}

// ListDevices 设备列表。
func (l *ListDevicesLogic) ListDevices(req *types.DeviceListReq) (*types.DeviceListResp, error) {
	out, err := l.svcCtx.Device.ListDevice(l.ctx, &dpb.ListDeviceReq{
		Keyword: req.Keyword, Status: req.Status, ProductKey: req.ProductKey,
		Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.DeviceListResp{Total: int(out.Total), List: []types.DeviceView{}}
	for _, d := range out.List {
		resp.List = append(resp.List, deviceView(d))
	}
	return resp, nil
}

// GetDevice 设备详情。
func (l *GetDeviceLogic) GetDevice(req *types.DeviceNoPath) (*types.DeviceGetResp, error) {
	out, err := l.svcCtx.Device.GetDevice(l.ctx, &dpb.GetDeviceReq{DeviceId: parseI64(req.DeviceId)})
	if err != nil {
		return nil, err
	}
	return &types.DeviceGetResp{Device: deviceView(out.Device)}, nil
}

// TransitionDevice 状态迁移（退役/报废白名单）。
func (l *TransitionDeviceLogic) TransitionDevice(req *types.DeviceTransitionReq) (*types.SimpleResp, error) {
	_, err := l.svcCtx.Device.Transition(l.ctx, &dpb.TransitionReq{
		DeviceId: parseI64(req.DeviceId), ToStatus: req.ToStatus, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ActivateDevice 激活（绑定客户 + 质保起算事件）。
func (l *ActivateDeviceLogic) ActivateDevice(req *types.DeviceActivateReq) (*types.SimpleResp, error) {
	_, err := l.svcCtx.Device.Activate(l.ctx, &dpb.ActivateReq{DeviceId: parseI64(req.DeviceId), PartyId: parseI64(req.PartyId)})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}

// ProvisionDevice 凭证补发（明文一次）。
func (l *ProvisionDeviceLogic) ProvisionDevice(req *types.DeviceNoPath) (*types.DeviceProvisionResp, error) {
	// 先取详情拿 SN
	d, err := l.svcCtx.Device.GetDevice(l.ctx, &dpb.GetDeviceReq{DeviceId: parseI64(req.DeviceId)})
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Device.ProvisionCredential(l.ctx, &dpb.ProvisionCredentialReq{Sn: d.Device.Sn})
	if err != nil {
		return nil, err
	}
	return &types.DeviceProvisionResp{Sn: d.Device.Sn, Secret: out.Secret}, nil
}

// GetDeviceShadow 设备影子。
func (l *GetDeviceShadowLogic) GetDeviceShadow(req *types.DeviceNoPath) (*types.DeviceShadowResp, error) {
	d, err := l.svcCtx.Device.GetDevice(l.ctx, &dpb.GetDeviceReq{DeviceId: parseI64(req.DeviceId)})
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Device.GetDeviceShadow(l.ctx, &dpb.GetDeviceShadowReq{Sn: d.Device.Sn})
	if err != nil {
		return nil, err
	}
	resp := &types.DeviceShadowResp{Sn: out.Sn, Ts: out.Ts, Raw: out.Raw, Metrics: []types.DeviceShadowMetric{}}
	for _, m := range out.Metrics {
		resp.Metrics = append(resp.Metrics, types.DeviceShadowMetric{Key: m.Key, Value: m.Value})
	}
	return resp, nil
}

// SendDeviceCmd 指令下发（控制类 step-up 由 authz SensitivePrefixes 强制）。
func (l *SendDeviceCmdLogic) SendDeviceCmd(req *types.CmdSendReq) (*types.CmdSendResp, error) {
	out, err := l.svcCtx.Device.SendCommand(l.ctx, &dpb.SendCommandReq{
		DeviceId: parseI64(req.DeviceId), Sn: req.Sn, CmdType: req.CmdType,
		ParamsJson: req.ParamsJson, ClientCmdId: req.ClientCmdId,
	})
	if err != nil {
		return nil, err
	}
	return &types.CmdSendResp{CmdId: i64s(out.CmdId)}, nil
}

// ListDeviceCmds 指令列表。
func (l *ListDeviceCmdsLogic) ListDeviceCmds(req *types.CmdListReq) (*types.CmdListResp, error) {
	out, err := l.svcCtx.Device.ListCmd(l.ctx, &dpb.ListCmdReq{
		DeviceId: parseI64(req.DeviceId), Sn: req.Sn, Status: req.Status,
		Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.CmdListResp{Total: int(out.Total), List: []types.CmdView{}}
	for _, c := range out.List {
		resp.List = append(resp.List, types.CmdView{
			CmdId: i64s(c.CmdId), DeviceId: i64s(c.DeviceId), Sn: c.Sn,
			CmdType: c.CmdType, ParamsJson: c.ParamsJson, Status: c.Status,
			RetryCount: int(c.RetryCount), AckedAt: c.AckedAt, FailReason: c.FailReason,
			Operator: i64s(c.Operator), CreatedAt: c.CreatedAt,
		})
	}
	return resp, nil
}

// ---- 拓扑 ----

// GetDeviceTopology 设备拓扑。
func (l *GetDeviceTopologyLogic) GetDeviceTopology(req *types.DeviceNoPath) (*types.DeviceTopologyResp, error) {
	out, err := l.svcCtx.Device.GetDeviceTopology(l.ctx, &dpb.GetDeviceTopologyReq{DeviceId: parseI64(req.DeviceId)})
	if err != nil {
		return nil, err
	}
	resp := &types.DeviceTopologyResp{Nodes: []types.DeviceTopologyNode{}}
	for _, n := range out.Nodes {
		resp.Nodes = append(resp.Nodes, types.DeviceTopologyNode{
			NodeId: i64s(n.NodeId), ParentId: i64s(n.ParentId),
			NodeType: n.NodeType, NodeName: n.NodeName, Sort: int(n.Sort),
		})
	}
	return resp, nil
}

// SaveDeviceTopology 全量替换。
func (l *SaveDeviceTopologyLogic) SaveDeviceTopology(req *types.DeviceTopologySaveReq) (*types.DeviceTopologySaveResp, error) {
	nodes := make([]*dpb.SaveTopologyNode, 0, len(req.Nodes))
	for _, n := range req.Nodes {
		nodes = append(nodes, &dpb.SaveTopologyNode{ParentRef: int64(n.ParentRef), NodeType: n.NodeType, NodeName: n.NodeName, Sort: int32(n.Sort)})
	}
	_, err := l.svcCtx.Device.SaveDeviceTopology(l.ctx, &dpb.SaveDeviceTopologyReq{DeviceId: parseI64(req.DeviceId), Nodes: nodes})
	if err != nil {
		return nil, err
	}
	return &types.DeviceTopologySaveResp{Saved: len(req.Nodes)}, nil
}

// ---- OTA ----

func firmwareView(f *dpb.FirmwareView) types.FirmwareView {
	return types.FirmwareView{
		FirmwareId: i64s(f.FirmwareId), ProductKey: f.ProductKey, Version: f.Version,
		FileUrl: f.FileUrl, FileSize: f.FileSize, Sha256: f.Sha256, SignAlg: f.SignAlg,
		Remark: f.Remark, CreatedAt: f.CreatedAt,
	}
}

// ListFirmwares 固件列表。
func (l *ListFirmwaresLogic) ListFirmwares(req *types.FirmwareListReq) (*types.FirmwareListResp, error) {
	out, err := l.svcCtx.Device.ListFirmware(l.ctx, &dpb.ListFirmwareReq{
		ProductKey: req.ProductKey, Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.FirmwareListResp{Total: int(out.Total), List: []types.FirmwareView{}}
	for _, f := range out.List {
		resp.List = append(resp.List, firmwareView(f))
	}
	return resp, nil
}

// SaveFirmware 固件登记（Ed25519 验签）。
func (l *SaveFirmwareLogic) SaveFirmware(req *types.FirmwareSaveReq) (*types.FirmwareSaveResp, error) {
	out, err := l.svcCtx.Device.SaveFirmware(l.ctx, &dpb.SaveFirmwareReq{
		ProductKey: req.ProductKey, Version: req.Version, FileUrl: req.FileUrl,
		FileSize: req.FileSize, Sha256: req.Sha256, Signature: req.Signature, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.FirmwareSaveResp{FirmwareId: i64s(out.FirmwareId)}, nil
}

func otaTaskView(t *dpb.OtaTaskView) types.OtaTaskView {
	out := types.OtaTaskView{
		TaskId: i64s(t.TaskId), TaskNo: t.TaskNo, Name: t.Name, ProductKey: t.ProductKey,
		FirmwareId: i64s(t.FirmwareId), RollbackFirmwareId: i64s(t.RollbackFirmwareId),
		BatchSize: int(t.BatchSize), FailThresholdPct: int(t.FailThresholdPct),
		Status: t.Status, Total: int(t.Total), SuccessCount: int(t.SuccessCount),
		FailCount: int(t.FailCount), FailReason: t.FailReason, CreatedAt: t.CreatedAt,
		Devices: []types.OtaDeviceView{},
	}
	return out
}

// appendOtaDevices 任务详情附带设备明细（GetOtaTask WithDevices 专用）。
func appendOtaDevices(out *types.OtaTaskView, devices []*dpb.OtaDeviceView) {
	for _, d := range devices {
		out.Devices = append(out.Devices, types.OtaDeviceView{
			Id: i64s(d.Id), DeviceId: i64s(d.DeviceId), Sn: d.Sn, Status: d.Status,
			CmdId: i64s(d.CmdId), RetryCount: int(d.RetryCount), Error: d.Error,
			DispatchedAt: d.DispatchedAt, FinishedAt: d.FinishedAt,
		})
	}
}

// ListOtaTasks OTA 任务列表。
func (l *ListOtaTasksLogic) ListOtaTasks(req *types.OtaTaskListReq) (*types.OtaTaskListResp, error) {
	out, err := l.svcCtx.Device.ListOtaTask(l.ctx, &dpb.ListOtaTaskReq{
		Status: req.Status, ProductKey: req.ProductKey, Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp := &types.OtaTaskListResp{Total: int(out.Total), List: []types.OtaTaskView{}}
	for _, t := range out.List {
		resp.List = append(resp.List, otaTaskView(t))
	}
	return resp, nil
}

// GetOtaTask OTA 任务详情（含设备明细）。
func (l *GetOtaTaskLogic) GetOtaTask(req *types.OtaTaskNoPath) (*types.OtaTaskView, error) {
	out, err := l.svcCtx.Device.GetOtaTask(l.ctx, &dpb.GetOtaTaskReq{TaskId: parseI64(req.TaskId), WithDevices: true})
	if err != nil {
		return nil, err
	}
	v := otaTaskView(out.Task)
	appendOtaDevices(&v, out.Devices)
	return &v, nil
}

// CreateOtaTask OTA 任务创建（灰度分批）。
func (l *CreateOtaTaskLogic) CreateOtaTask(req *types.OtaTaskCreateReq) (*types.OtaTaskCreateResp, error) {
	deviceIds := make([]int64, 0, len(req.DeviceIds))
	for _, id := range req.DeviceIds {
		deviceIds = append(deviceIds, parseI64(id))
	}
	out, err := l.svcCtx.Device.CreateOtaTask(l.ctx, &dpb.CreateOtaTaskReq{
		Name: req.Name, ProductKey: req.ProductKey,
		FirmwareId: parseI64(req.FirmwareId), RollbackFirmwareId: parseI64(req.RollbackFirmwareId),
		BatchSize: int32(req.BatchSize), DeviceIds: deviceIds,
	})
	if err != nil {
		return nil, err
	}
	return &types.OtaTaskCreateResp{TaskId: i64s(out.TaskId)}, nil
}

// RollbackOtaTask OTA 回滚。
func (l *RollbackOtaTaskLogic) RollbackOtaTask(req *types.OtaTaskRollbackReq) (*types.OtaTaskRollbackResp, error) {
	out, err := l.svcCtx.Device.RollbackOtaTask(l.ctx, &dpb.RollbackOtaTaskReq{TaskId: parseI64(req.TaskId), Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return &types.OtaTaskRollbackResp{RolledBack: int(out.RolledBack)}, nil
}

var _ = context.Background

// ---- Logic 结构体（goctl 骨架等价物；重新生成 goctl 时删除 *_logic.go 保留本文件）----

type ImportDevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewImportDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ImportDevicesLogic {
	return &ImportDevicesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListDevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDevicesLogic {
	return &ListDevicesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceLogic {
	return &GetDeviceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type TransitionDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransitionDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransitionDeviceLogic {
	return &TransitionDeviceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ActivateDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewActivateDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActivateDeviceLogic {
	return &ActivateDeviceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ProvisionDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProvisionDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProvisionDeviceLogic {
	return &ProvisionDeviceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetDeviceShadowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDeviceShadowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceShadowLogic {
	return &GetDeviceShadowLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SendDeviceCmdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendDeviceCmdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendDeviceCmdLogic {
	return &SendDeviceCmdLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListDeviceCmdsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListDeviceCmdsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListDeviceCmdsLogic {
	return &ListDeviceCmdsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetDeviceTopologyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetDeviceTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDeviceTopologyLogic {
	return &GetDeviceTopologyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SaveDeviceTopologyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveDeviceTopologyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveDeviceTopologyLogic {
	return &SaveDeviceTopologyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListFirmwaresLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFirmwaresLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFirmwaresLogic {
	return &ListFirmwaresLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type SaveFirmwareLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveFirmwareLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveFirmwareLogic {
	return &SaveFirmwareLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type ListOtaTasksLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListOtaTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOtaTasksLogic {
	return &ListOtaTasksLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type GetOtaTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOtaTaskLogic {
	return &GetOtaTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type CreateOtaTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOtaTaskLogic {
	return &CreateOtaTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

type RollbackOtaTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRollbackOtaTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackOtaTaskLogic {
	return &RollbackOtaTaskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
