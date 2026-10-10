// cron 扫描任务体（S6-01，ADR-09）：
//   - cmd-retry-scan：PENDING/SENT 且 next_exec_at 到期 → 重投（≤max_retry，超限 FAILED + cmd_failed 事件）；
//   - cmd-stale-scan：TimingWheel 失联兜底（SENT 超时未 ACK 回 PENDING）；
//   - ota-dispatch-scan：RUNNING 任务按 batch_size 分批下发（灰度），失败率超阈自动 PAUSED（FR-IOT-007）。

package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"micro-server/services/device/internal/model"
	"micro-server/services/device/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// ScanCmdRetry 指令重试扫描（TimingWheel 失联/重启后的持久兜底，E16）。
func ScanCmdRetry(ctx context.Context, sc *svc.ServiceContext) error {
	now := time.Now()
	due, err := sc.Models.Cmd.FindRetryDue(ctx, now, 200)
	if err != nil {
		return err
	}
	for _, c := range due {
		// 无租户扫描 → 按行租户重建 ctx（下游/事件租户口径）
		cctx := tenantx.WithTenant(context.WithoutCancel(context.Background()), c.TenantId)
		if c.RetryCount >= c.MaxRetry {
			// 超限 → FAILED + cmd_failed（ops 告警，FR-IOT-006）
			err := sc.Conn.TransactCtx(cctx, func(ctx context.Context, session sqlx.Session) error {
				n, err := sc.Models.Cmd.CasStatusTx(ctx, session, c.TenantId, c.CmdId, c.Status, "FAILED", "重试超限（设备无响应）")
				if err != nil || n == 0 {
					return err
				}
				return eventbus.Emit(ctx, session, cmdFailedEvent(c.TenantId, c.DeviceId, c.Sn, c.CmdType, "重试超限"))
			})
			if err != nil {
				logx.WithContext(ctx).Errorf("cmd FAILED 终态失败 cmd_id=%d: %v", c.CmdId, err)
			}
			continue
		}
		// 重投（SENT→PENDING + retry+1 + next_exec_at 退避）
		backoff := time.Duration(effectiveCmdBackoffSec(sc)) * time.Second
		if err := sc.Models.Cmd.IncRetryTx(cctx, nil, c.CmdId, time.Now().Add(backoff)); err != nil {
			logx.WithContext(ctx).Errorf("cmd 重试推进失败 cmd_id=%d: %v", c.CmdId, err)
			continue
		}
		if err := dispatchCmd(cctx, sc, c.TenantId, c.CmdId, c.Sn, c.CmdType, paramStr(c)); err != nil {
			logx.WithContext(ctx).Errorf("cmd 重投失败（下轮继续）cmd_id=%d: %v", c.CmdId, err)
			continue
		}
		// 重投后重新挂 ACK 轮
		watchAck(sc, c.TenantId, c.CmdId, time.Duration(effectiveAckTimeoutMs(sc))*time.Millisecond)
	}
	if len(due) > 0 {
		logx.Infof("cmd retry scan: due=%d", len(due))
	}
	return nil
}

// ScanOtaDispatch OTA 分批下发（灰度推进 + 失败率超阈 PAUSED；断点续传 = 只扫 PENDING/FAILED 行）。
func ScanOtaDispatch(ctx context.Context, sc *svc.ServiceContext) error {
	tasks, err := sc.Models.OtaTask.ListRunningOrPaused(ctx, 50)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.Status != "RUNNING" {
			continue
		}
		tctx := tenantx.WithTenant(context.WithoutCancel(context.Background()), t.TenantId)
		batch := int(t.BatchSize)
		if cc := confcenterCurrent().OtaDispatchBatch; cc > 0 && cc < batch {
			batch = cc
		}
		pending, err := sc.Models.OtaDevice.ListDispatchable(tctx, t.TaskId, batch)
		if err != nil {
			return err
		}
		success, failed := 0, 0
		for _, d := range pending {
			cmdId, derr := dispatchOtaDevice(tctx, sc, t, d)
			if derr != nil {
				failed++
				logx.WithContext(tctx).Errorf("ota 下发失败 task=%s sn=%s: %v", t.TaskNo, d.Sn, derr)
				continue
			}
			success++
			_ = cmdId
		}
		if success+failed > 0 {
			if err := sc.Models.OtaTask.IncProgressTx(tctx, nil, t.TaskId, success, failed); err != nil {
				logx.WithContext(tctx).Errorf("ota 进度推进失败 task=%s: %v", t.TaskNo, err)
			}
			// 失败率超阈自动 PAUSED（FR-IOT-007）
			total := t.SuccessCount + t.FailCount + int64(success+failed)
			if total > 0 {
				failPct := int((t.FailCount + int64(failed)) * 100 / total)
				if failPct >= int(t.FailThresholdPct) && failPct > 0 {
					if _, err := sc.Models.OtaTask.CasStatus(tctx, t.TenantId, t.TaskId, "RUNNING", "PAUSED",
						truncateErr(fmt.Errorf("失败率 %d%% 超阈 %d%%，自动暂停", failPct, t.FailThresholdPct))); err == nil {
						if err := eventbus.Emit(tctx, sc.Conn, otaPausedEvent(t.TenantId, t.TaskId, t.TaskNo, "失败率超阈", failPct)); err != nil {
							logx.WithContext(tctx).Errorf("ota_paused 事件失败 task=%s: %v", t.TaskNo, err)
						}
						logx.Errorf("ota task auto-paused task_no=%s fail_pct=%d%%", t.TaskNo, failPct)
					}
				}
			}
			logx.Infof("ota dispatch: task=%s sent=%d failed=%d", t.TaskNo, success, failed)
		}
		// 完成判定：全部终态 → DONE
		counts, err := sc.Models.OtaDevice.CountByStatus(tctx, t.TaskId)
		if err == nil && counts["PENDING"] == 0 && counts["SENT"] == 0 {
			if _, err := sc.Models.OtaTask.CasStatus(tctx, t.TenantId, t.TaskId, "RUNNING", "DONE", ""); err == nil {
				logx.Infof("ota task done task_no=%s", t.TaskNo)
			}
		}
	}
	return nil
}

// dispatchOtaDevice 单设备升级指令（OTA_UPGRADE 复用指令链路：MQTT + ACK 轮 + cron 兜底）。
func dispatchOtaDevice(ctx context.Context, sc *svc.ServiceContext, t *model.OtaTask, d *model.OtaDevice) (int64, error) {
	fw, err := sc.Models.Firmware.FindOneScoped(ctx, t.TenantId, t.FirmwareId)
	if err != nil {
		return 0, err
	}
	cmdId := sc.Snowflake.MustNextID()
	paramsRaw, _ := json.Marshal(map[string]any{
		"firmware_url": fw.FileUrl, "sha256": fw.Sha256, "version": fw.Version, "sign_alg": fw.SignAlg,
	})
	params := sql.NullString{String: string(paramsRaw), Valid: true}
	op := int64(0)
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Cmd.InsertTx(ctx, session, &model.Cmd{
			CmdId: cmdId, DeviceId: d.DeviceId, Sn: d.Sn, CmdType: "OTA_UPGRADE",
			Params: params, Status: "PENDING", MaxRetry: 3,
			NextExecAt: time.Now(), TenantId: t.TenantId, Operator: toNullInt64(op),
		}); err != nil {
			return err
		}
		return sc.Models.OtaDevice.MarkSentTx(ctx, session, d.Id, cmdId, time.Now())
	})
	if err != nil {
		return 0, err
	}
	if err := dispatchCmd(ctx, sc, t.TenantId, cmdId, d.Sn, "OTA_UPGRADE", params.String); err != nil {
		return cmdId, err
	}
	watchAck(sc, t.TenantId, cmdId, time.Duration(effectiveAckTimeoutMs(sc))*time.Millisecond)
	return cmdId, nil
}

func paramStr(c *model.Cmd) string {
	if c.Params.Valid {
		return c.Params.String
	}
	return ""
}

func effectiveAckTimeoutMs(sc *svc.ServiceContext) int64 {
	if v := confcenterCurrent().CmdAckTimeoutMs; v > 0 {
		return v
	}
	return sc.Config.CmdAckTimeoutMs
}

func effectiveCmdBackoffSec(sc *svc.ServiceContext) int {
	if v := confcenterCurrent().CmdRetryBackoffSec; v > 0 {
		return v
	}
	return sc.Config.CmdRetryBackoffSec
}

// CountOverdueCmd 人工观测口径（cron 注册表对账）。
func CountOverdueCmd(ctx context.Context, sc *svc.ServiceContext) int64 {
	due, err := sc.Models.Cmd.FindRetryDue(ctx, time.Now(), 1000)
	if err != nil {
		return 0
	}
	overdue := int64(0)
	for _, c := range due {
		if c.RetryCount >= c.MaxRetry {
			overdue++
		}
	}
	return overdue
}
