// Package cronx · ops 定时任务注册表（ADR-09：延迟任务 = 状态字段 + next_exec_at/期限列 + 扫描）。
package cronx

import (
	"context"
	"time"

	"micro-server/services/ops/internal/logic"
	"micro-server/services/ops/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// RegisterAll 登记 ops 全部扫描任务（main 启动期调用一次）。
func RegisterAll(sc *svc.ServiceContext) {
	// 工单接单超时升级（DispatchTicket.accept_deadline 扫描；状态不变，释放受理人回待派）
	cronxRegister("ticket_accept_timeout", 30*time.Second, func(ctx context.Context) error {
		tickets, err := sc.Models.Ticket.ScanAcceptTimeoutAll(ctx, time.Now(), 100)
		if err != nil {
			return err
		}
		for _, tk := range tickets {
			if err := logic.EscalateTicket(ctx, sc, tk, logic.AcceptTimeoutMin(sc)); err != nil {
				logx.Errorf("cronx: 接单超时升级失败 ticket=%d: %v", tk.TicketId, err)
			}
		}
		return nil
	})

	// SLA 超时分级升级（sla_timer 到期扫描；响应/解决双口径）
	cronxRegister("sla_breach_scan", 30*time.Second, func(ctx context.Context) error {
		return logic.ScanSlaBreaches(ctx, sc)
	})

	// 巡检任务生成（inspection_plan.next_exec_at 扫描；@daily/@weekly/@monthly/@h:N）
	cronxRegister("inspection_task_generate", time.Minute, func(ctx context.Context) error {
		plans, err := sc.Models.InsPlan.ScanDueAll(ctx, time.Now(), 50)
		if err != nil {
			return err
		}
		for _, plan := range plans {
			if err := logic.GenerateTasksForPlan(sc, plan); err != nil {
				logx.Errorf("cronx: 巡检任务生成失败 plan=%d: %v", plan.PlanId, err)
			}
		}
		return nil
	})
}

// cronxRegister 包装 logic 包侧 Register（避免循环引用命名冲突）。
func cronxRegister(name string, interval time.Duration, run func(ctx context.Context) error) {
	Register(Task{Name: name, Interval: interval, Run: run})
}
