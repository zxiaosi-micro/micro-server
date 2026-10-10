// OTA logic（S6-01：SaveFirmware/ListFirmware/CreateOtaTask/GetOtaTask/ListOtaTask/RollbackOtaTask）。
// 固件 Ed25519 验签（未签名拒绝入库，FR-IOT-007）；灰度分批下发；失败率超阈自动 PAUSED；回滚重推旧固件。
package logic

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// SaveFirmware 固件登记（sha256 + Ed25519 签名强校验）。
func (l *SaveFirmwareLogic) SaveFirmware(in *pb.SaveFirmwareReq) (*pb.SaveFirmwareResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.ProductKey == "" || in.Version == "" || in.FileUrl == "" {
		return nil, errProductRequired
	}
	if len(in.Sha256) != 64 {
		return nil, errFwShaBad
	}
	if _, err := hex.DecodeString(in.Sha256); err != nil {
		return nil, errFwShaBad
	}
	if len(l.svcCtx.OtaPubKey) != ed25519.PublicKeySize {
		return nil, errFwSignNotReady
	}
	sig, err := base64.StdEncoding.DecodeString(in.Signature)
	if err != nil {
		return nil, errFwSignBad
	}
	// 验签对象 = sha256 hex 串（产线用私钥对 hash hex 签名；密钥 keygen 产出，FR-IOT-007）
	if !ed25519.Verify(ed25519.PublicKey(l.svcCtx.OtaPubKey), []byte(strings.ToLower(in.Sha256)), sig) {
		return nil, errFwSignBad
	}
	op := opUID(l.ctx)
	fwId := l.svcCtx.Snowflake.MustNextID()
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		return l.svcCtx.Models.Firmware.InsertTx(ctx, session, &model.Firmware{
			FirmwareId: fwId,
			ProductKey: in.ProductKey,
			Version:    in.Version,
			FileUrl:    in.FileUrl,
			FileSize:   in.FileSize,
			Sha256:     strings.ToLower(in.Sha256),
			Signature:  in.Signature,
			SignAlg:    "Ed25519",
			Remark:     toNullString(in.Remark),
			TenantId:   tid,
			CreatedBy:  toNullInt64(op),
		})
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errFwSignBad.WithMsg("同产品同版本固件已存在")
		}
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("firmware saved pk=%s ver=%s sha=%s by=%d", in.ProductKey, in.Version, in.Sha256[:12], op)
	return &pb.SaveFirmwareResp{FirmwareId: fwId}, nil
}

// ListFirmware 固件列表。
func (l *ListFirmwareLogic) ListFirmware(in *pb.ListFirmwareReq) (*pb.ListFirmwareResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Firmware.ListPage(l.ctx, tid, in.ProductKey, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListFirmwareResp{Total: total, List: []*pb.FirmwareView{}}
	for _, f := range list {
		v := &pb.FirmwareView{
			FirmwareId: f.FirmwareId, ProductKey: f.ProductKey, Version: f.Version,
			FileUrl: f.FileUrl, FileSize: f.FileSize, Sha256: f.Sha256, SignAlg: f.SignAlg,
			CreatedAt: f.CreatedAt.UnixMilli(),
		}
		if f.Remark.Valid {
			v.Remark = f.Remark.String
		}
		resp.List = append(resp.List, v)
	}
	return resp, nil
}

// CreateOtaTask 创建 OTA 任务（灰度分批：CREATED 即启用扫描器分批下发）。
func (l *CreateOtaTaskLogic) CreateOtaTask(in *pb.CreateOtaTaskReq) (*pb.CreateOtaTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	fw, err := l.svcCtx.Models.Firmware.FindOneScoped(l.ctx, tid, in.FirmwareId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errFwNotFound
		}
		return nil, err
	}
	if in.RollbackFirmwareId > 0 {
		if _, err := l.svcCtx.Models.Firmware.FindOneScoped(l.ctx, tid, in.RollbackFirmwareId); err != nil {
			if err == model.ErrNotFound {
				return nil, errFwNotFound
			}
			return nil, err
		}
	}
	// 目标设备解析（灰度清单 or 产品全量 ACTIVATED）
	var targets []*model.Device
	if len(in.DeviceIds) > 0 {
		targets, err = l.svcCtx.Models.Device.ListByIds(l.ctx, in.DeviceIds)
	} else {
		targets, err = l.svcCtx.Models.Device.ListActiveByProduct(l.ctx, in.ProductKey, 10000)
	}
	if err != nil {
		return nil, err
	}
	// 过滤：产品匹配 + 已激活
	filtered := targets[:0:0]
	for _, d := range targets {
		if d.ProductKey == fw.ProductKey && d.Status == "ACTIVATED" {
			filtered = append(filtered, d)
		}
	}
	if len(filtered) == 0 {
		return nil, errOtaNoTarget
	}
	batchSize := int64(in.BatchSize)
	if batchSize <= 0 {
		batchSize = 50
	}
	op := opUID(l.ctx)
	taskId := l.svcCtx.Snowflake.MustNextID()
	taskNo := fmt.Sprintf("OTA%d", taskId)

	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := l.svcCtx.Models.OtaTask.InsertTx(ctx, session, &model.OtaTask{
			TaskId:             taskId,
			TaskNo:             taskNo,
			Name:               in.Name,
			ProductKey:         fw.ProductKey,
			FirmwareId:         fw.FirmwareId,
			RollbackFirmwareId: toNullInt64(in.RollbackFirmwareId),
			BatchSize:          batchSize,
			FailThresholdPct:   20,
			Status:             "RUNNING",
			Total:              int64(len(filtered)),
			TenantId:           tid,
			CreatedBy:          toNullInt64(op),
		}); err != nil {
			return err
		}
		for _, d := range filtered {
			if err := l.svcCtx.Models.OtaDevice.InsertTx(ctx, session, &model.OtaDevice{
				Id:       l.svcCtx.Snowflake.MustNextID(),
				TaskId:   taskId,
				DeviceId: d.DeviceId,
				Sn:       d.Sn,
				Status:   "PENDING",
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
	logx.WithContext(l.ctx).Infof("ota task created task_no=%s targets=%d batch=%d", taskNo, len(filtered), batchSize)
	return &pb.CreateOtaTaskResp{TaskId: taskId}, nil
}

// GetOtaTask 任务详情（含明细）。
func (l *GetOtaTaskLogic) GetOtaTask(in *pb.GetOtaTaskReq) (*pb.GetOtaTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	t, err := l.svcCtx.Models.OtaTask.FindOneScoped(l.ctx, tid, in.TaskId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errOtaTaskNotFound
		}
		return nil, err
	}
	resp := &pb.GetOtaTaskResp{Task: otaTaskView(t)}
	if in.WithDevices {
		devices, err := l.svcCtx.Models.OtaDevice.ListByTask(l.ctx, tid, in.TaskId)
		if err != nil {
			return nil, err
		}
		resp.Devices = []*pb.OtaDeviceView{}
		for _, d := range devices {
			v := &pb.OtaDeviceView{
				Id: d.Id, DeviceId: d.DeviceId, Sn: d.Sn, Status: d.Status,
				RetryCount: int32(d.RetryCount),
			}
			if d.CmdId.Valid {
				v.CmdId = d.CmdId.Int64
			}
			if d.Error.Valid {
				v.Error = d.Error.String
			}
			if d.DispatchedAt.Valid {
				v.DispatchedAt = d.DispatchedAt.Time.UnixMilli()
			}
			if d.FinishedAt.Valid {
				v.FinishedAt = d.FinishedAt.Time.UnixMilli()
			}
			resp.Devices = append(resp.Devices, v)
		}
	}
	return resp, nil
}

// ListOtaTask 任务列表。
func (l *ListOtaTaskLogic) ListOtaTask(in *pb.ListOtaTaskReq) (*pb.ListOtaTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.OtaTask.ListPage(l.ctx, tid, in.Status, in.ProductKey, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListOtaTaskResp{Total: total, List: []*pb.OtaTaskView{}}
	for _, t := range list {
		resp.List = append(resp.List, otaTaskView(t))
	}
	return resp, nil
}

// RollbackOtaTask 回滚：任务置 ROLLED_BACK + 明细复位 PENDING（扫描器按 rollback 固件重推）。
func (l *RollbackOtaTaskLogic) RollbackOtaTask(in *pb.RollbackOtaTaskReq) (*pb.RollbackOtaTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	t, err := l.svcCtx.Models.OtaTask.FindOneScoped(l.ctx, tid, in.TaskId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errOtaTaskNotFound
		}
		return nil, err
	}
	if !t.RollbackFirmwareId.Valid {
		return nil, errOtaRollbackMiss
	}
	switch t.Status {
	case "RUNNING", "PAUSED", "DONE":
	default:
		return nil, errOtaStatusBad
	}
	if n, err := l.svcCtx.Models.OtaTask.CasStatus(l.ctx, tid, in.TaskId, t.Status, "ROLLED_BACK",
		truncateErr(fmt.Errorf("回滚：%s", in.Reason))); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, errOtaStatusBad
	}
	rolled, err := l.svcCtx.Models.OtaDevice.ResetForRollback(l.ctx, in.TaskId)
	if err != nil {
		return nil, err
	}
	// 进度对账留痕（回滚后重推重新累计）
	logx.WithContext(l.ctx).Infof("ota rollback task_id=%d reset done", in.TaskId)
	logx.WithContext(l.ctx).Infof("ota task rolled back task_no=%s rolled=%d by=%d", t.TaskNo, rolled, opUID(l.ctx))
	return &pb.RollbackOtaTaskResp{RolledBack: rolled}, nil
}

func otaTaskView(t *model.OtaTask) *pb.OtaTaskView {
	v := &pb.OtaTaskView{
		TaskId: t.TaskId, TaskNo: t.TaskNo, Name: t.Name, ProductKey: t.ProductKey,
		FirmwareId: t.FirmwareId, BatchSize: int32(t.BatchSize),
		FailThresholdPct: int32(t.FailThresholdPct), Status: t.Status,
		Total: int32(t.Total), SuccessCount: int32(t.SuccessCount), FailCount: int32(t.FailCount),
		CreatedAt: t.CreatedAt.UnixMilli(),
	}
	if t.RollbackFirmwareId.Valid {
		v.RollbackFirmwareId = t.RollbackFirmwareId.Int64
	}
	if t.FailReason.Valid {
		v.FailReason = t.FailReason.String
	}
	return v
}

var _ = time.Second
