// 巡检内核：station_created 自动建计划 / cron 生成任务 / 批量提交转工单（FR-OPS-007）。
package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/internal/svc"

	stationclient "micro-server/services/station/station"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

// planFromStationCreated station_created 事件 → 自动建计划（幂等：同站同名计划查回）。
// 模板：租户默认日巡模板（无则建）；周期 @daily；next_exec_at = 次日 08:00。
func planFromStationCreated(sc *svc.ServiceContext, tenantId, stationId int64, stationName string) {
	ctx := tenantx.WithTenant(context.Background(), tenantId)
	if stationName == "" {
		if st, err := sc.Station.GetStation(ctx, &stationclient.GetStationReq{StationId: stationId}); err == nil && st.Station != nil {
			stationName = st.Station.Name
		}
	}
	if stationName == "" {
		logx.Errorf("ops: station_created 场站不存在 tenant=%d station=%d（跳过建计划）", tenantId, stationId)
		return
	}

	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		// 幂等：重放事件直接命中已有计划
		if _, err := sc.Models.InsPlan.FindByStationNameTx(ctx, session, tenantId, stationId, "日常巡检"); err == nil {
			return nil
		} else if err != model.ErrNotFound {
			return err
		}
		// 默认模板（租户内不存在才建）
		tmpl, err := sc.Models.InsTmpl.FindDefault(ctx, tenantId)
		if err == model.ErrNotFound {
			tmpl = &model.InspectionTemplate{
				TemplateId:  sc.Snowflake.MustNextID(),
				Name:        "默认日巡模板",
				ItemsSchema: model.DefaultInspectionSchema,
				TenantId:    tenantId,
			}
			if err := sc.Models.InsTmpl.InsertTx(ctx, session, tmpl); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		// 计划（次日 08:00 起生成）
		next := time.Now().AddDate(0, 0, 1)
		next = time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, next.Location())
		return sc.Models.InsPlan.InsertTx(ctx, session, &model.InspectionPlan{
			PlanId:      sc.Snowflake.MustNextID(),
			StationId:   stationId,
			StationName: stationName,
			Name:        "日常巡检",
			TemplateId:  tmpl.TemplateId,
			CronExpr:    "@daily",
			Enabled:     1,
			NextExecAt:  next,
			TenantId:    tenantId,
		})
	})
	if err != nil {
		logx.Errorf("ops: 巡检计划创建失败 tenant=%d station=%d: %v", tenantId, stationId, err)
		return
	}
	logx.Infof("ops: 巡检计划就绪 tenant=%d station=%d(%s)", tenantId, stationId, stationName)
}

// nextExecAt 简化 cron 集合推下次执行（@daily/@weekly/@monthly/@h:N）。
func nextExecAt(from time.Time, cronExpr string) (time.Time, error) {
	dayStart := time.Date(from.Year(), from.Month(), from.Day(), 8, 0, 0, 0, from.Location())
	switch {
	case cronExpr == "@daily":
		n := dayStart.AddDate(0, 0, 1)
		if !n.After(from) {
			n = n.AddDate(0, 0, 1)
		}
		return n, nil
	case cronExpr == "@weekly":
		n := dayStart.AddDate(0, 0, 7)
		if !n.After(from) {
			n = n.AddDate(0, 0, 7)
		}
		return n, nil
	case cronExpr == "@monthly":
		n := dayStart.AddDate(0, 1, 0)
		if !n.After(from) {
			n = n.AddDate(0, 1, 0)
		}
		return n, nil
	case len(cronExpr) > 3 && cronExpr[:3] == "@h:":
		h, aerr := strconv.Atoi(cronExpr[3:])
		if aerr != nil || h < 1 || h > 168 {
			return time.Time{}, errInspectCronBad
		}
		return from.Add(time.Duration(h) * time.Hour), nil
	default:
		return time.Time{}, errInspectCronBad
	}
}

// generateTasksForPlan cron 扫描驱动：到期计划生成任务（同计划同时刻幂等，ADR-09）。
func generateTasksForPlan(sc *svc.ServiceContext, plan *model.InspectionPlan) error {
	ctx := tenantx.WithTenant(context.Background(), plan.TenantId)
	now := time.Now()
	scheduledFor := plan.NextExecAt
	task := &model.InspectionTask{
		TaskId:       sc.Snowflake.MustNextID(),
		PlanId:       plan.PlanId,
		TemplateId:   plan.TemplateId,
		StationId:    plan.StationId,
		StationName:  plan.StationName,
		Title:        plan.StationName + " · " + plan.Name + " " + scheduledFor.Format("01-02 15:04"),
		ScheduledFor: scheduledFor,
		Status:       "PENDING",
		TenantId:     plan.TenantId,
	}
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.InsTask.InsertTx(ctx, session, task); err != nil {
			return err
		}
		next, err := nextExecAt(now, plan.CronExpr)
		if err != nil {
			return err
		}
		return sc.Models.InsPlan.MarkExeced(ctx, plan.TenantId, plan.PlanId, now, next)
	})
	if err != nil {
		// uk 冲突 = 并发重放，视为已生成
		logx.Errorf("ops: 巡检任务生成失败 plan=%d: %v", plan.PlanId, err)
		return err
	}
	logx.Infof("ops: 巡检任务已生成 task=%d plan=%d station=%d", task.TaskId, plan.PlanId, plan.StationId)
	return nil
}

// submitInspectionCore 批量提交明细；异常项（anomalous）生成 INSPECTION 工单并回填
// （FR-OPS-007：巡检异常项自动生成工单）。
func submitInspectionCore(ctx context.Context, sc *svc.ServiceContext, tid, taskId, uid int64, location, photos, remark string, items []InspectionItemIn) (int64, string, error) {
	task, err := sc.Models.InsTask.FindOneScoped(ctx, tid, taskId)
	if err != nil {
		return 0, "", errInspectNotFound
	}
	if task.Status != "PENDING" {
		return 0, "", errInspectSubmitted
	}
	if len(items) == 0 {
		return 0, "", errItemsRequired
	}

	var ticketIds []int64
	anomalyCount := int64(0)
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, it := range items {
			item := &model.InspectionItem{
				ItemId:    sc.Snowflake.MustNextID(),
				TaskId:    taskId,
				Name:      it.Name,
				Result:    nullString(it.Result),
				Anomalous: boolToInt64(it.Anomalous),
				TenantId:  tid,
			}
			if err := sc.Models.InsItem.InsertTx(ctx, session, item); err != nil {
				return err
			}
			if !it.Anomalous {
				continue
			}
			anomalyCount++
			// 异常项直生成工单（type=INSPECTION）
			tk := &model.Ticket{
				TicketId:         sc.Snowflake.MustNextID(),
				TicketNo:         "TK" + time.Now().Format("20060102") + fmt.Sprintf("%06d", sc.Snowflake.MustNextID()%1000000),
				Type:             "INSPECTION",
				Title:            "巡检异常：" + it.Name + "（" + task.StationName + "）",
				Content:          "巡检任务 " + task.Title + " 异常项：" + it.Name + "；结果：" + it.Result,
				Priority:         "P2",
				Status:           "PENDING_DISPATCH",
				StationId:        task.StationId,
				StationName:      task.StationName,
				InspectionItemId: item.ItemId,
				CreatedBy:        nullInt64(uid),
				ClientActionId:   nullString("insp:" + strconv.FormatInt(item.ItemId, 10)),
				TenantId:         tid,
			}
			if err := sc.Models.Ticket.InsertTx(ctx, session, tk); err != nil {
				return err
			}
			if err := sc.Models.TicketLog.InsertTx(ctx, session, &model.TicketLog{
				LogId: sc.Snowflake.MustNextID(), TicketId: tk.TicketId,
				Action: "create", ToStatus: "PENDING_DISPATCH",
				OperatorUid: uid, Note: "巡检异常项自动生成", TenantId: tid,
			}); err != nil {
				return err
			}
			if err := sc.Models.InsItem.FillTicketTx(ctx, session, item.ItemId, tk.TicketId); err != nil {
				return err
			}
			ticketIds = append(ticketIds, tk.TicketId)
		}
		return nil
	})
	if err != nil {
		return 0, "", err
	}

	tidsJSON, _ := json.Marshal(ticketIds)
	ok, err := sc.Models.InsTask.CasSubmit(ctx, tid, taskId, uid, location, photos, remark, anomalyCount, string(tidsJSON))
	if err != nil {
		return 0, "", err
	}
	if !ok {
		return 0, "", errInspectSubmitted
	}
	logx.Infof("ops: 巡检提交 task=%d 异常=%d 工单=%v", taskId, anomalyCount, ticketIds)
	return anomalyCount, string(tidsJSON), nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
