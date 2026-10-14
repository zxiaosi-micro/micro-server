// logic 公共助手（未读计数/通知外发/配置访问点）。
package logic

import (
	"context"
	"strconv"

	"micro-server/services/ops/internal/confcenter"
	"micro-server/services/ops/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/eventbus"
)

func confcenterCurrent() confcenter.OpsConf { return confcenter.Current() }

// acceptTimeoutMin 接单超时（confcenter 热调 → 服务配置兜底）。
func acceptTimeoutMin(sc *svc.ServiceContext) int {
	if v := confcenterCurrent().AcceptTimeoutMin; v > 0 {
		return v
	}
	if v := sc.Config.AcceptTimeoutMin; v > 0 {
		return v
	}
	return 30
}

// redisKeyAlertUnread 告警未读计数键（租户+场站维度；DB0）。
func redisKeyAlertUnread(tenantId, stationId int64) string {
	return "micro:ops:alert:unread:" + strconv.FormatInt(tenantId, 10) + ":" + strconv.FormatInt(stationId, 10)
}

// bumpAlertUnread 新告警入库后自增（聚合命中不涨：已计入的未读不变）。
func bumpAlertUnread(sc *svc.ServiceContext, tenantId, stationId int64) {
	if sc.Rd == nil {
		return
	}
	key := redisKeyAlertUnread(tenantId, stationId)
	if _, err := sc.Rd.IncrCtx(context.Background(), key); err != nil {
		logx.Errorf("ops: 未读计数自增失败 key=%s: %v", key, err)
	}
}

// refreshAlertUnread 确认/关闭后按 DB 重算回填（对账兜底，避免计数漂移）。
func refreshAlertUnread(sc *svc.ServiceContext, tenantId, stationId int64) {
	if sc.Rd == nil {
		return
	}
	n, err := sc.Models.Alert.CountOpen(context.Background(), tenantId, stationId)
	if err != nil {
		logx.Errorf("ops: 未读计数回填失败 tenant=%d station=%d: %v", tenantId, stationId, err)
		return
	}
	key := redisKeyAlertUnread(tenantId, stationId)
	if n == 0 {
		_, _ = sc.Rd.DelCtx(context.Background(), key)
		return
	}
	_ = sc.Rd.SetexCtx(context.Background(), key, strconv.FormatInt(n, 10), 86400)
}

// getAlertUnread 未读计数：Redis miss → DB count 回填（最终一致可接受）。
func getAlertUnread(sc *svc.ServiceContext, tenantId, stationId int64) int64 {
	if sc.Rd != nil {
		key := redisKeyAlertUnread(tenantId, stationId)
		if v, err := sc.Rd.GetCtx(context.Background(), key); err == nil && v != "" {
			if n, cerr := strconv.ParseInt(v, 10, 64); cerr == nil {
				return n
			}
		}
	}
	n, err := sc.Models.Alert.CountOpen(context.Background(), tenantId, stationId)
	if err != nil {
		logx.Errorf("ops: 未读计数直查失败: %v", err)
		return 0
	}
	if n > 0 {
		refreshAlertUnread(sc, tenantId, stationId)
	}
	return n
}

// getTicketActive 工单活跃计数（低频场景直查 DB；读多写少无需 Redis）。
func getTicketActive(sc *svc.ServiceContext, tenantId, stationId int64) int64 {
	n, err := sc.Models.Ticket.CountActive(context.Background(), tenantId, stationId, 0, "")
	if err != nil {
		logx.Errorf("ops: 工单活跃计数失败: %v", err)
		return 0
	}
	return n
}

// notify 持久通知（notification_request 事件写 Outbox；无推送组件，消费方为 notification）。
// 通知允许最终一致：独立写 Outbox，不与业务同事务。
func notify(sc *svc.ServiceContext, tenantId, userId int64, templateCode, bizType, bizId string, params map[string]string) {
	if userId <= 0 {
		return
	}
	// Payload 直接传 map（[]byte 会被信封 base64 编码，消费方 parse 失败——S7 冒烟实测）
	if err := eventbus.Emit(context.Background(), sc.Conn, eventbus.EmitInput{
		Topic:    eventbus.TopicNotificationRequest,
		Type:     eventbus.TypeNotificationRequest,
		Key:      bizId,
		TenantID: tenantId,
		Payload: map[string]any{
			"user_id": userId, "template_code": templateCode,
			"params": params, "biz_type": bizType, "biz_id": bizId,
		},
	}); err != nil {
		logx.Errorf("ops: notification_request 写 Outbox 失败 biz=%s: %v", bizId, err)
	}
}
