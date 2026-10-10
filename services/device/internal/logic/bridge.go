// 事件消费 handler（S6-01 状态机）：stock.in / stock.out / cmd.ack。
// 幂等口径：生命周期日志唯一键（device_id+event_type+event_id）承接重放（1062 静默跳过），
// 业务推进 CAS 守卫——重放安全（at-least-once 正解，02 §9.3）。

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
)

// snsPayload 库存事件载荷（inventory stockInEvent/stockOutEvent 对齐）。
type snsPayload struct {
	TenantId    int64    `json:"tenant_id"`
	WarehouseId int64    `json:"warehouse_id"`
	SkuId       int64    `json:"sku_id"`
	Qty         int64    `json:"qty"`
	BizNo       string   `json:"biz_no"`
	Sns         []string `json:"sns"`
}

// HandleStockIn stock.in → PRODUCED/OUT→IN_STOCK（入库上架）。
func HandleStockIn(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p snsPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("stock.in 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		if len(p.Sns) == 0 {
			return nil // 非设备类 SKU 入库
		}
		for _, sn := range p.Sns {
			if err := transitionByEvent(ctx, sc, tid, sn, "IN_STOCK", env.EventType, env.EventID,
				fmt.Sprintf("入库 %s", p.BizNo)); err != nil {
				return err
			}
		}
		logx.WithContext(ctx).Infof("stock.in 状态推进 sns=%v biz=%s", p.Sns, p.BizNo)
		return nil
	}
}

// HandleStockOut stock.out → IN_STOCK/ACTIVATED→OUT（出库 + order_no 回填）。
func HandleStockOut(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p snsPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("stock.out 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		if len(p.Sns) == 0 {
			return nil
		}
		for _, sn := range p.Sns {
			if err := transitionByEvent(ctx, sc, tid, sn, "OUT", env.EventType, env.EventID,
				fmt.Sprintf("出库 %s", p.BizNo)); err != nil {
				return err
			}
			// 回填来源订单号（biz_no 形如 ORD...:skuId，取前段）
			orderNo := p.BizNo
			for i := 0; i < len(orderNo); i++ {
				if orderNo[i] == ':' {
					orderNo = orderNo[:i]
					break
				}
			}
			if orderNo != "" {
				if err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
					d, err := sc.Models.Device.FindOneBySnScoped(ctx, tid, sn)
					if err != nil {
						return err
					}
					d.OrderNo = sql.NullString{String: orderNo, Valid: true}
					return sc.Models.Device.UpdateTx(ctx, session, d)
				}); err != nil {
					logx.WithContext(ctx).Errorf("order_no 回填失败 sn=%s: %v", sn, err)
				}
			}
		}
		logx.WithContext(ctx).Infof("stock.out 状态推进 sns=%v biz=%s", p.Sns, p.BizNo)
		return nil
	}
}

// HandleCmdAck cmd.ack → ACKED（设备回执；iotingest 消费 MQTT up/{tenant}/{pk}/{sn}/ack 后转发本事件）。
func HandleCmdAck(sc *svc.ServiceContext) eventbus.Handler {
	return func(ctx context.Context, env *eventbus.Envelope) error {
		var p struct {
			TenantId int64  `json:"tenant_id"`
			Sn       string `json:"sn"`
			CmdId    string `json:"cmd_id"`
			Ok       bool   `json:"ok"`
			Reason   string `json:"reason"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return fmt.Errorf("cmd.ack 载荷解析失败: %w", err)
		}
		tid := env.TenantID
		var cmdId int64
		if _, err := fmt.Sscanf(p.CmdId, "%d", &cmdId); err != nil {
			logx.WithContext(ctx).Errorf("cmd.ack cmd_id 非法: %s", p.CmdId)
			return nil // 畸形回执不重投
		}
		err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			if p.Ok {
				_, err := sc.Models.Cmd.MarkAckedTx(ctx, session, tid, cmdId, time.Now())
				return err
			}
			// 设备明确拒绝：CAS PENDING|SENT→FAILED + cmd.failed 事件（ops 告警口径）
			n, err := sc.Models.Cmd.CasStatusTx(ctx, session, tid, cmdId, "SENT", "FAILED", p.Reason)
			if err != nil {
				return err
			}
			if n == 0 {
				n, err = sc.Models.Cmd.CasStatusTx(ctx, session, tid, cmdId, "PENDING", "FAILED", p.Reason)
				if err != nil {
					return err
				}
			}
			if n > 0 {
				c, err := sc.Models.Cmd.FindOneScoped(ctx, tid, cmdId)
				if err != nil {
					return err
				}
				return eventbus.Emit(ctx, session, cmdFailedEvent(tid, c.DeviceId, c.Sn, c.CmdType, p.Reason))
			}
			return nil
		})
		if err != nil {
			return err
		}
		logx.WithContext(ctx).Infof("cmd.ack 处理完成 cmd_id=%d ok=%v", cmdId, p.Ok)
		return nil
	}
}

// transitionByEvent 事件驱动状态迁移（CAS 守卫 + 生命周期日志；重放安全）。
func transitionByEvent(ctx context.Context, sc *svc.ServiceContext, tid int64, sn, to, eventType, eventId, remark string) error {
	// 事件驱动白名单（FR-DEV-003：状态只由事件驱动）
	var froms []string
	switch to {
	case "IN_STOCK":
		froms = []string{"PRODUCED", "IN_STOCK"} // IN_STOCK 重放幂等
	case "OUT":
		froms = []string{"IN_STOCK", "OUT"}
	case "ACTIVATED":
		froms = []string{"OUT", "ACTIVATED"}
	default:
		return nil
	}
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		d, err := sc.Models.Device.FindOneBySnScoped(ctx, tid, sn)
		if err != nil {
			if err == model.ErrNotFound {
				logx.WithContext(ctx).Errorf("事件目标设备不存在（跳过）sn=%s event=%s", sn, eventType)
				return nil
			}
			return err
		}
		matched := false
		for _, from := range froms {
			if d.Status == from {
				matched = true
				break
			}
		}
		if !matched {
			return nil // 状态不匹配（乱序事件）：静默跳过，日志留痕
		}
		if _, err := sc.Models.Device.CasStatusTx(ctx, session, tid, d.DeviceId, d.Status, to); err != nil {
			return err
		}
		return lifecycleAppendTx(ctx, session, sc, d.DeviceId, d.Sn, d.Status, to, eventType, eventId, remark, tid, 0)
	})
	if err != nil && isDupKey(err) {
		return nil // 生命周期日志唯一键兜底（事件重放）
	}
	return err
}
