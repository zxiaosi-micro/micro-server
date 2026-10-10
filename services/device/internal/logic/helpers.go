// device logic 公共助手（S6-01）。
package logic

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/internal/svc"
	"micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/tenantx"
	aupb "micro-server/services/audit/pb"
)

// opUID 操作人 uid（未注入返回 0）。
func opUID(ctx context.Context) int64 { return ctxkit.UID(ctx) }

// sqlTime *time.Time → sql.NullTime。
func sqlTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// truncateErr 错误串截断（入库 512 列）。
func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 480 {
		s = s[:480]
	}
	return s
}

// genDeviceSecret 生成 32B base64url 设备密钥（FR-IOT-002 一机一密）。
func genDeviceSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// lifecycleAppendTx 生命周期日志追加（幂等键 1062 静默跳过——事件重放安全）。
func lifecycleAppendTx(ctx context.Context, session sqlx.Session, sc *svc.ServiceContext,
	deviceId int64, sn, from, to, eventType, eventId, remark string, tenantId, op int64) error {
	err := sc.Models.LifecycleLog.InsertTx(ctx, session, &model.DeviceLifecycleLog{
		LogId:      sc.Snowflake.MustNextID(),
		DeviceId:   deviceId,
		Sn:         sn,
		FromStatus: from,
		ToStatus:   to,
		EventType:  eventType,
		EventId:    eventId,
		Remark:     toNullString(remark),
		TenantId:   tenantId,
		CreatedBy:  toNullInt64(op),
	})
	if err != nil && isDupKey(err) {
		return nil // 事件重放：日志已存在
	}
	return err
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

// deviceView 行 → proto 视图（secret 明文不出库，has_secret 表意）。
func deviceView(d *model.Device) *pb.DeviceView {
	v := &pb.DeviceView{
		DeviceId:   d.DeviceId,
		Sn:         d.Sn,
		ProductKey: d.ProductKey,
		Model:      d.Model,
		BatchNo:    d.BatchNo,
		Status:     d.Status,
		PartyId:    d.PartyId.Int64,
		CreatedAt:  d.CreatedAt.UnixMilli(),
	}
	if d.OrderNo.Valid {
		v.OrderNo = d.OrderNo.String
	}
	if d.ActivatedAt.Valid {
		v.ActivatedAt = d.ActivatedAt.Time.UnixMilli()
	}
	v.HasSecret = d.DeviceSecret.Valid && d.DeviceSecret.String != ""
	return v
}

// tenantxWith 按租户重建 ctx（cron/消费跨租户扫描后调下游用）。
func tenantxWith(ctx context.Context, tid int64) context.Context {
	return tenantx.WithTenant(ctx, tid)
}

// deviceActivatedEvent device.activated 事件（contract 兜底起算质保）。
func deviceActivatedEvent(tid int64, sn string, activatedAt int64) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic:    eventbus.TopicDeviceActivated,
		Type:     eventbus.TypeDeviceActivated,
		Key:      sn,
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "sn": sn, "activated_at": activatedAt,
		},
	}
}

// cmdFailedEvent cmd.failed 事件（ops 告警口径 FR-IOT-006 指令失败告警）。
func cmdFailedEvent(tid int64, deviceId int64, sn, cmdType, reason string) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic:    eventbus.TopicCmdFailed,
		Type:     eventbus.TypeCmdFailed,
		Key:      sn,
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "device_id": deviceId, "sn": sn,
			"cmd_type": cmdType, "reason": reason,
		},
	}
}

// otaPausedEvent ota.paused 事件（失败率超阈自动暂停，FR-IOT-007）。
func otaPausedEvent(tid int64, taskId int64, taskNo, reason string, failPct int) eventbus.EmitInput {
	return eventbus.EmitInput{
		Topic:    eventbus.TopicOtaPaused,
		Type:     eventbus.TypeOtaPaused,
		Key:      taskNo,
		TenantID: tid,
		Payload: map[string]any{
			"tenant_id": tid, "task_id": taskId, "task_no": taskNo,
			"reason": reason, "fail_pct": failPct,
		},
	}
}

// writeCmdAudit 指令独立审计（audit.WriteCmdLog；未配置跳过——cmd 表留痕兜底，100% 审计口径 FR-IOT-006）。
func writeCmdAudit(ctx context.Context, sc *svc.ServiceContext, cmdId int64, sn, cmdType, params, result string, op int64, tid int64) {
	if sc.Audit == nil {
		return
	}
	go func(ctx context.Context) {
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if _, err := sc.Audit.WriteCmdLog(cctx, &aupb.WriteCmdLogReq{
			CmdId:       fmt.Sprintf("%d", cmdId),
			Sn:          sn,
			Action:      cmdType,
			PayloadJson: params,
			Result:      result,
			Uid:         op,
			TenantId:    tid,
		}); err != nil {
			logx.WithContext(ctx).Errorf("cmd_audit 写入失败（cmd 表留痕兜底）cmd_id=%d: %v", cmdId, err)
		}
	}(ctx)
}
