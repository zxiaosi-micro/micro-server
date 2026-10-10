// p3smoke · 运维域全链冒烟（S7 阶段验收：告警闭环/工单/接单超时扫描升级/SLA 分级升级/巡检转工单）。
//
// 链路：alert_candidate(Kafka 裸载荷) → ops 判定聚合 → PendingAlerts 未读 → ack → 转工单
// → dispatch → accept → start → submit → verify → close alert → archive；
// 接单超时升级（accept_deadline 扫描，ADR-09 无 MQ）；SLA 到期分级升级（注入 0 分钟策略快照）；
// station_created 建计划 → 巡检任务生成 → 异常项自动转工单；MOCKWX 通知 outbound_log 触达。
//
// 前置：compose core、mqinit、ops/party/station/notification/identity 进程运行中；
//
//	通知模板/channel 由本工具幂等种子（micro_notification）。
//
// 纪律：严禁并发执行（E1）。
//
//	env:
//	  source tools/devenv.sh micro_ops   # MICRO_DSN_OPS + MICRO_DEV_* 注入
//	  go run ./tools/p3smoke
package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	_ "github.com/go-sql-driver/mysql"

	oppb "micro-server/services/ops/pb"
)

const (
	tenantID = 9000000000000000001
	seedUID  = 9000000000000000002
)

var (
	kafkaBrokers = envOr("KAFKA_BROKERS", "127.0.0.1:29092")
	opsRPC       = envOr("OPS_RPC", "127.0.0.1:8094")
	dsn          = envOr("MICRO_DSN_OPS", "")
)

// dsnNotification 通知库种子（同实例不同库）。
var dsnNotification = replaceDB(dsn, "micro_notification")

func main() {
	logx.MustSetup(logx.LogConf{ServiceName: "p3smoke", Mode: "console"})
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), seedUID)

	if dsn == "" {
		fail("MICRO_DSN_OPS 未设置（source tools/devenv.sh micro_ops）")
	}

	// ---- 直连 ops RPC ----
	conn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{opsRPC},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
	ops := oppb.NewOpsClient(conn)

	db, err := sql.Open("mysql", dsn)
	must(err, "ops db open")
	ndb, err := sql.Open("mysql", dsnNotification)
	must(err, "notification db open")
	defer func() { _ = db.Close(); _ = ndb.Close() }()

	sn := fmt.Sprintf("P3-%d", time.Now().UnixMilli()%100000000)
	fmt.Printf("== p3smoke 运维域冒烟 sn=%s ==\n", sn)

	// ---- 0. 种子：通知模板 + WECHAT 渠道（幂等）----
	seedNotification(ndb)

	// ---- 1. alert_candidate → 告警产生（判定/聚合/未读计数）----
	candidateRaw, _ := json.Marshal(map[string]any{
		"tenant_id": tenantID, "sn": sn, "product_key": "ESS-DEMO",
		"metric": "cell_temp", "value": 65.5, "comparator": "gt", "threshold": 55,
		"level": "P1", "source": "telemetry", "occurred_at": time.Now().UnixMilli(),
	})
	publishRaw("alert_candidate", sn, candidateRaw)
	var alertID int64
	var alertLevel string
	waitFor(45*time.Second, time.Second, "告警产生（PendingAlerts 含本 sn）", func() (bool, error) {
		out, err := ops.PendingAlerts(ctx, &oppb.PendingAlertsReq{SinceId: 0, Limit: 50})
		if err != nil {
			return false, err
		}
		for _, it := range out.Items {
			// 按 sn 匹配本冒烟告警（库中可能有历史遗留告警，取最后一条防串）
			if strings.Contains(it.Title, sn) {
				alertID = it.AlertId
				alertLevel = it.Level
			}
		}
		return alertID > 0, nil
	})
	pass("告警产生 alert=%d level=%s", alertID, alertLevel)

	// 聚合验证：同源重复候选 → aggregate_count 累计
	publishRaw("alert_candidate", sn, candidateRaw)
	waitFor(10*time.Second, 500*time.Millisecond, "告警聚合（aggregate_count≥2）", func() (bool, error) {
		a, err := ops.GetAlert(ctx, &oppb.GetAlertReq{AlertId: alertID})
		if err != nil {
			return false, err
		}
		return a.Alert.AggregateCount >= 2, nil
	})
	pass("同源候选聚合累计（抑制窗口）")

	// ---- 2. 确认 → 转工单 → 派单 → 接单 → 开工 → 提交 → 验收 → 关单 → 告警关闭/归档 ----
	_, err = ops.AckAlert(ctx, &oppb.AckAlertReq{AlertId: alertID})
	must(err, "AckAlert")
	pass("告警确认")

	tr, err := ops.TransferAlertToTicket(ctx, &oppb.TransferAlertToTicketReq{AlertId: alertID})
	must(err, "TransferAlertToTicket")
	pass("转工单 ticket=%d(%s) 优先级=P1(隐含确认)", tr.TicketId, tr.TicketNo)

	_, err = ops.DispatchTicket(ctx, &oppb.DispatchTicketReq{TicketId: tr.TicketId, AssigneeUid: seedUID, AcceptTimeoutMin: 10})
	must(err, "DispatchTicket")
	pass("派单 → seed 账号（deadline 10min）")

	_, err = ops.AcceptTicket(ctx, &oppb.AcceptTicketReq{TicketId: tr.TicketId})
	must(err, "AcceptTicket")
	_, err = ops.StartTicket(ctx, &oppb.StartTicketReq{TicketId: tr.TicketId})
	must(err, "StartTicket")
	_, err = ops.SubmitTicket(ctx, &oppb.SubmitTicketReq{TicketId: tr.TicketId, Result: "更换风扇，温度回落"})
	must(err, "SubmitTicket")
	pass("接单→开工→提交")

	_, err = ops.VerifyTicket(ctx, &oppb.VerifyTicketReq{TicketId: tr.TicketId, Pass: true})
	must(err, "VerifyTicket")
	pass("验收通过（SLA resolved 打点）")

	_, err = ops.CloseTicket(ctx, &oppb.CloseTicketReq{TicketId: tr.TicketId})
	must(err, "CloseTicket")
	pass("工单关闭")

	_, err = ops.CloseAlert(ctx, &oppb.CloseAlertReq{AlertId: alertID, TicketId: tr.TicketId})
	must(err, "CloseAlert")
	waitFor(5*time.Second, 300*time.Millisecond, "告警 CLOSED", func() (bool, error) {
		a, err := ops.GetAlert(ctx, &oppb.GetAlertReq{AlertId: alertID})
		if err != nil {
			return false, err
		}
		return a.Alert.Status == "CLOSED", nil
	})
	_, err = ops.ArchiveAlert(ctx, &oppb.ArchiveAlertReq{AlertId: alertID})
	must(err, "ArchiveAlert")
	waitFor(5*time.Second, 300*time.Millisecond, "告警 ARCHIVED", func() (bool, error) {
		a, err := ops.GetAlert(ctx, &oppb.GetAlertReq{AlertId: alertID})
		if err != nil {
			return false, err
		}
		return a.Alert.Status == "ARCHIVED", nil
	})
	pass("告警关闭→归档（闭环完成）")

	// ---- 3. 接单超时扫描升级（ADR-09：escalate_at/accept_deadline，无 MQ 延迟消息）----
	tk, err := ops.CreateTicket(ctx, &oppb.CreateTicketReq{Type: "STATION", Title: "p3smoke 接单超时验证", Priority: "P2"})
	must(err, "CreateTicket(escalate)")
	_, err = ops.DispatchTicket(ctx, &oppb.DispatchTicketReq{TicketId: tk.TicketId, AssigneeUid: seedUID, AcceptTimeoutMin: 1})
	must(err, "DispatchTicket(1min)")
	waitFor(100*time.Second, 3*time.Second, "接单超时扫描升级（escalate_level≥1，≤100s）", func() (bool, error) {
		g, err := ops.GetTicket(ctx, &oppb.GetTicketReq{TicketId: tk.TicketId})
		if err != nil {
			return false, err
		}
		return g.Ticket.EscalateLevel >= 1, nil
	})
	pass("接单超时升级（扫描驱动）")
	_, err = ops.CancelTicket(ctx, &oppb.CancelTicketReq{TicketId: tk.TicketId, Reason: "冒烟完成"})
	must(err, "CancelTicket")

	// ---- 4. SLA 到期分级升级（注入 0 分钟策略快照计时器）----
	tk2, err := ops.CreateTicket(ctx, &oppb.CreateTicketReq{Type: "AFTER_SALE", Title: "p3smoke SLA 升级验证", Priority: "P1"})
	must(err, "CreateTicket(sla)")
	injectSLATimer(db, tk2.TicketId)
	waitFor(50*time.Second, 3*time.Second, "SLA 超时扫描（escalate_level≥1，≤50s）", func() (bool, error) {
		var lvl int
		if err := db.QueryRow("select escalate_level from sla_timer where ticket_id = ?", tk2.TicketId).Scan(&lvl); err != nil {
			return false, err
		}
		return lvl >= 1, nil
	})
	pass("SLA 分级升级（快照冻结 + 扫描）")
	_, err = ops.CancelTicket(ctx, &oppb.CancelTicketReq{TicketId: tk2.TicketId, Reason: "冒烟完成"})
	must(err, "CancelTicket")

	// ---- 5. 巡检：建计划 → 任务生成 → 异常项自动转工单 ----
	plan, err := ops.CreateInspectionPlan(ctx, &oppb.CreateInspectionPlanReq{
		StationId: 0, Name: "p3smoke-plan-" + fmt.Sprint(time.Now().Unix()%100000),
		CronExpr: "@daily",
	})
	must(err, "CreateInspectionPlan")
	// 冒烟加速：next_exec_at 置为现在（cron 扫描 60s 内生成任务）
	_, err = db.Exec("update inspection_plan set next_exec_at = now(3) where plan_id = ?", plan.PlanId)
	must(err, "加速 next_exec_at")
	var taskID int64
	waitFor(80*time.Second, 3*time.Second, "巡检任务生成（cron 扫描）", func() (bool, error) {
		return db.QueryRow("select task_id from inspection_task where plan_id = ? limit 1", plan.PlanId).Scan(&taskID) == nil, nil
	})
	pass("巡检任务已生成 task=%d", taskID)

	sub, err := ops.SubmitInspectionTask(ctx, &oppb.SubmitInspectionTaskReq{
		TaskId: taskID,
		Items: []*oppb.InspectionItemSubmit{
			{Name: "外观检查", Result: "{\"value\":\"ok\"}", Anomalous: false},
			{Name: "连接紧固", Result: "{\"value\":\"松动\"}", Anomalous: true},
		},
		Location: "116.397,39.908",
	})
	must(err, "SubmitInspectionTask")
	if sub.AnomalyCount != 1 || sub.TicketIds == "" {
		fail("巡检异常项未转工单 anomaly=%d ids=%s", sub.AnomalyCount, sub.TicketIds)
	}
	var insTicketID int64
	_, _ = fmt.Sscanf(sub.TicketIds, "[%d]", &insTicketID)
	g, err := ops.GetTicket(ctx, &oppb.GetTicketReq{TicketId: insTicketID})
	must(err, "GetTicket(inspection)")
	if g.Ticket.Type != "INSPECTION" {
		fail("异常工单类型 = %s, want INSPECTION", g.Ticket.Type)
	}
	pass("巡检异常项自动转工单 ticket=%d type=INSPECTION", insTicketID)
	_, err = ops.CancelTicket(ctx, &oppb.CancelTicketReq{TicketId: insTicketID, Reason: "冒烟完成"})
	must(err, "CancelTicket(inspection)")

	// ---- 6. MOCKWX 通知触达（outbound_log wechat 行）----
	waitFor(15*time.Second, time.Second, "MOCKWX 订阅消息触达（outbound_log channel=wechat）", func() (bool, error) {
		var n int
		if err := ndb.QueryRow("select count(*) from outbound_log where channel = 'WECHAT'").Scan(&n); err != nil {
			return false, err
		}
		return n >= 1, nil
	})
	pass("微信订阅消息 dev mock 触达")

	fmt.Printf("== p3smoke 全链通过 ==\n")
}

// ---- 种子与注入 ----

// seedNotification 通知模板/渠道幂等种子（dev 租户；WECHAT 渠道走 mock Provider）。
func seedNotification(db *sql.DB) {
	_, err := db.Exec(`insert ignore into notify_channel (channel_id, code, provider, config, status, tenant_id)
		values (1, 'WECHAT', 'wechat', '{}', 1, ?)`, tenantID)
	must(err, "seed notify_channel")
	templates := []struct{ code, title, content string }{
		{"alert_created", "【{{level}}】{{title}}", "场站告警：{{title}}"},
		{"ticket_dispatched", "新工单派单 {{ticket_no}}", "{{title}}，请及时接单"},
		{"ticket_closed", "工单已关闭 {{ticket_no}}", "处理完成"},
		{"sla_escalated", "SLA {{what}}升级 {{ticket_no}}", "升级至第 {{level}} 级"},
	}
	for i, t := range templates {
		_, err := db.Exec(`insert ignore into notify_template (template_id, code, title_template, content_template, channel, status, tenant_id)
			values (?, ?, ?, ?, 'WECHAT', 1, ?)`, int64(100+i), t.code, t.title, t.content, tenantID)
		must(err, "seed notify_template "+t.code)
	}
	pass("通知模板/渠道种子就绪（alert_created/ticket_dispatched/...）")
}

// injectSLATimer 注入 0 分钟策略快照计时器（到期即扫描升级；快照冻结口径演示）。
func injectSLATimer(db *sql.DB, ticketID int64) {
	snap, _ := json.Marshal(map[string]any{
		"strategy_id": 1, "code": "smoke", "name": "冒烟策略", "level": "P1",
		"response_minutes": 0, "resolve_minutes": 0,
	})
	_, err := db.Exec(`insert ignore into sla_timer
		(timer_id, ticket_id, strategy_id, strategy_snapshot, response_due_at, resolve_due_at, tenant_id)
		values (?, ?, 1, ?, now(3), now(3), ?)`, time.Now().UnixNano(), ticketID, string(snap), tenantID)
	must(err, "inject sla_timer")
}

// ---- 基础设施 ----

func publishRaw(topic, key string, raw []byte) {
	w := &kafka.Writer{Addr: kafka.TCP(kafkaBrokers), Topic: topic, AllowAutoTopicCreation: false}
	if err := w.WriteMessages(context.Background(), kafka.Message{Key: []byte(key), Value: raw}); err != nil {
		fail("Kafka 生产 %s 失败: %v", topic, err)
	}
	_ = w.Close()
}

func replaceDB(dsn, newDB string) string {
	// micro_app:pw@tcp(host:port)/db?params → 换库名
	for i := 0; i < len(dsn); i++ {
		if dsn[i] == '/' {
			rest := dsn[i+1:]
			for j := 0; j < len(rest); j++ {
				if rest[j] == '?' {
					return dsn[:i+1] + newDB + rest[j:]
				}
			}
			return dsn[:i+1] + newDB
		}
	}
	return dsn
}

func waitFor(total time.Duration, interval time.Duration, name string, probe func() (bool, error)) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		ok, err := probe()
		if ok {
			return
		}
		_ = err
		time.Sleep(interval)
	}
	fail("等待超时: %s", name)
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "p3smoke FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(format string, args ...any) {
	fmt.Printf("  ✅ "+format+"\n", args...)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "p3smoke FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var _ = hex.EncodeToString
