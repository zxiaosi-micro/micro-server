// 设备档案/状态机/凭证 logic（S6-01：ImportSN/GetDevice/ListDevice/ListByOrderNo/Transition/Activate/ProvisionCredential）。
package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

// 状态机白名单（FR-DEV-003：其余状态只由事件驱动，禁止人工直改）。
var transitionAllow = map[string][]string{
	"RETIRED":  {"ACTIVATED"},
	"SCRAPPED": {"IN_STOCK", "OUT", "ACTIVATED", "RETIRED"},
}

const importMaxBatch = 1000

// ImportSN 批量导入（产线口径）：SN 唯一 + 自动写 EMQX 凭证 + secrets CSV（明文仅一次）。
func (l *ImportSNLogic) ImportSN(in *pb.ImportSNReq) (*pb.ImportSNResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if len(in.Items) == 0 {
		return nil, errImportEmpty
	}
	if len(in.Items) > importMaxBatch {
		return nil, errImportTooMany
	}
	if l.svcCtx.Encryptor == nil {
		return nil, errEncryptNotReady
	}
	op := opUID(l.ctx)

	resp := &pb.ImportSNResp{Secrets: []*pb.SecretRow{}, ProvisionErrors: []string{}}
	for _, item := range in.Items {
		if item.Sn == "" {
			return nil, errSnRequired
		}
		if item.ProductKey == "" {
			return nil, errProductRequired
		}
	}

	for _, item := range in.Items {
		// 产线明文 secret（仅出现在本响应）
		secret, err := genDeviceSecret()
		if err != nil {
			return nil, err
		}
		enc, err := l.svcCtx.Encryptor.Encrypt([]byte(secret))
		if err != nil {
			return nil, err
		}

		deviceId := l.svcCtx.Snowflake.MustNextID()
		err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
			if err := l.svcCtx.Models.Device.InsertTx(ctx, session, &model.Device{
				DeviceId:     deviceId,
				Sn:           item.Sn,
				ProductKey:   item.ProductKey,
				Model:        item.Model,
				BatchNo:      item.BatchNo,
				DeviceSecret: sql.NullString{String: enc, Valid: true},
				Status:       "PRODUCED",
				TenantId:     tid,
				CreatedBy:    toNullInt64(op),
				UpdatedBy:    toNullInt64(op),
			}); err != nil {
				return err
			}
			// 生命周期首笔：PRODUCED（事件源 = 导入动作本身）
			return lifecycleAppendTx(ctx, session, l.svcCtx, deviceId, item.Sn, "", "PRODUCED",
				"device.imported", fmt.Sprintf("import:%d", deviceId), "", tid, op)
		})
		if err != nil {
			if isDupKey(err) {
				resp.ProvisionErrors = append(resp.ProvisionErrors, item.Sn+": SN 已存在（跳过）")
				continue
			}
			return nil, err
		}
		resp.Imported++

		// 自动写 EMQX 一机一密（FR-DEV-001 导入即有凭证；幂等，失败可补发）
		if in.Provision {
			if err := l.svcCtx.Emqx.ProvisionDevice(l.ctx, item.Sn, secret, tid, item.ProductKey); err != nil {
				logx.WithContext(l.ctx).Errorf("ProvisionCredential 失败 sn=%s: %v", item.Sn, err)
				resp.ProvisionErrors = append(resp.ProvisionErrors, fmt.Sprintf("%s: %v", item.Sn, truncateErr(err)))
				continue
			}
		}
		resp.Secrets = append(resp.Secrets, &pb.SecretRow{Sn: item.Sn, Secret: secret})
	}
	return resp, nil
}

// GetDevice 设备详情。
func (l *GetDeviceLogic) GetDevice(in *pb.GetDeviceReq) (*pb.GetDeviceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	d, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	return &pb.GetDeviceResp{Device: deviceView(d)}, nil
}

// ListDevice 设备列表。
func (l *ListDeviceLogic) ListDevice(in *pb.ListDeviceReq) (*pb.ListDeviceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	if in.Status != "" && !isDeviceStatus(in.Status) {
		return nil, errStatusBad
	}
	list, total, err := l.svcCtx.Models.Device.ListPage(l.ctx, tid, in.Keyword, in.Status, in.ProductKey, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListDeviceResp{Total: total, List: []*pb.DeviceView{}}
	for _, d := range list {
		resp.List = append(resp.List, deviceView(d))
	}
	return resp, nil
}

func isDeviceStatus(s string) bool {
	switch s {
	case "PRODUCED", "IN_STOCK", "OUT", "ACTIVATED", "RETIRED", "SCRAPPED":
		return true
	}
	return false
}

// ListByOrderNo 按来源订单查设备。
func (l *ListByOrderNoLogic) ListByOrderNo(in *pb.ListByOrderNoReq) (*pb.ListByOrderNoResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Device.ListByOrderNoByTenant(l.ctx, tid, in.OrderNo)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListByOrderNoResp{List: []*pb.DeviceView{}}
	for _, d := range list {
		resp.List = append(resp.List, deviceView(d))
	}
	return resp, nil
}

// Transition 白名单状态迁移（ACTIVATED→RETIRED / RETIRED→SCRAPPED / →SCRAPPED）。
func (l *TransitionLogic) Transition(in *pb.TransitionReq) (*pb.TransitionResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if !isDeviceStatus(in.ToStatus) {
		return nil, errStatusBad
	}
	d, err := l.svcCtx.Models.Device.FindOneScoped(l.ctx, tid, in.DeviceId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	allowed := false
	for _, from := range transitionAllow[in.ToStatus] {
		if d.Status == from {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errTransitionBad
	}
	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		n, err := l.svcCtx.Models.Device.CasStatusTx(ctx, session, tid, in.DeviceId, d.Status, in.ToStatus)
		if err != nil {
			return err
		}
		if n == 0 {
			return errTransitionBad
		}
		return lifecycleAppendTx(ctx, session, l.svcCtx, in.DeviceId, d.Sn, d.Status, in.ToStatus,
			"manual.transition", fmt.Sprintf("transition:%d:%d", in.DeviceId, time.Now().UnixMilli()), in.Reason, tid, op)
	})
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("device transition sn=%s %s→%s by=%d", d.Sn, d.Status, in.ToStatus, op)
	return &pb.TransitionResp{}, nil
}

// Activate 激活（OUT→ACTIVATED）：绑定客户 + device_activated 事件（contract 兜底起算质保）。
func (l *ActivateLogic) Activate(in *pb.ActivateReq) (*pb.ActivateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
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
	if d.Status == "ACTIVATED" {
		return nil, errDeviceActivated
	}
	if d.Status != "OUT" {
		return nil, errDeviceNotOut
	}
	op := opUID(l.ctx)
	now := time.Now()
	activatedAt := now.UnixMilli()

	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		n, err := l.svcCtx.Models.Device.CasStatusTx(ctx, session, tid, d.DeviceId, "OUT", "ACTIVATED")
		if err != nil {
			return err
		}
		if n == 0 {
			return errTransitionBad
		}
		// 绑定客户 + 激活时刻
		d.Status = "ACTIVATED"
		d.PartyId = toNullInt64(in.PartyId)
		d.ActivatedAt = sqlTime(&now)
		if err := l.svcCtx.Models.Device.UpdateTx(ctx, session, d); err != nil {
			return err
		}
		return lifecycleAppendTx(ctx, session, l.svcCtx, d.DeviceId, d.Sn, "OUT", "ACTIVATED",
			"device.activated", fmt.Sprintf("activate:%d:%d", d.DeviceId, activatedAt), "", tid, op)
	})
	if err != nil {
		return nil, err
	}

	// device_activated 事件（独立事务 Emit：激活已提交，事件由 Outbox Relay 投递）
	if err := eventbus.Emit(l.ctx, l.svcCtx.Conn, deviceActivatedEvent(tid, d.Sn, activatedAt)); err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("device activated sn=%s party=%d", d.Sn, in.PartyId)
	return &pb.ActivateResp{}, nil
}

// ProvisionCredential 补发/重建设备凭证（E6：EMQX bootstrap 丢失后重放；明文一次返回）。
func (l *ProvisionCredentialLogic) ProvisionCredential(in *pb.ProvisionCredentialReq) (*pb.ProvisionCredentialResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Sn == "" {
		return nil, errSnRequired
	}
	if l.svcCtx.Encryptor == nil {
		return nil, errEncryptNotReady
	}
	d, err := l.svcCtx.Models.Device.FindOneBySnScoped(l.ctx, tid, in.Sn)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	secret, err := genDeviceSecret()
	if err != nil {
		return nil, err
	}
	enc, err := l.svcCtx.Encryptor.Encrypt([]byte(secret))
	if err != nil {
		return nil, err
	}
	op := opUID(l.ctx)
	err = l.svcCtx.Conn.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		d.DeviceSecret = sql.NullString{String: enc, Valid: true}
		d.UpdatedBy = toNullInt64(op)
		return l.svcCtx.Models.Device.UpdateTx(ctx, session, d)
	})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Emqx.ProvisionDevice(l.ctx, d.Sn, secret, tid, d.ProductKey); err != nil {
		return nil, errcodeMap(err)
	}
	return &pb.ProvisionCredentialResp{Secret: secret}, nil
}

// errcodeMap EMQX 错误映射业务码。
func errcodeMap(err error) error {
	if err == nil {
		return nil
	}
	_ = json.Marshal // 保持 json 引用
	return errProvisionFailed.WithCause(err)
}

// GetDeviceBySn 按 SN 查询（station 绑定归一/激活扫码用）。
func (l *GetDeviceBySnLogic) GetDeviceBySn(in *pb.GetDeviceBySnReq) (*pb.GetDeviceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Sn == "" {
		return nil, errSnRequired
	}
	d, err := l.svcCtx.Models.Device.FindOneBySnScoped(l.ctx, tid, in.Sn)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errDeviceNotFound
		}
		return nil, err
	}
	return &pb.GetDeviceResp{Device: deviceView(d)}, nil
}
