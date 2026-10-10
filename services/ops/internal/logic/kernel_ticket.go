// 工单内核：创建（离线幂等）/派单（区域+技能匹配）/状态机迁移/SLA 计时挂接
// （FR-OPS-004/005/006/008；接单超时升级在 cronx 扫描，ADR-09）。
package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"micro-server/services/ops/internal/model"
	"micro-server/services/ops/internal/svc"

	contractclient "micro-server/services/contract/contract"
	partyclient "micro-server/services/party/party"
	stationclient "micro-server/services/station/station"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

// 工单类型白名单（FR-OPS-005）。
var ticketTypes = map[string]bool{
	"ALERT": true, "AFTER_SALE": true, "INSPECTION": true, "INSTALL": true, "STATION": true,
}

// 工单状态机白名单（from → 允许的 to；终态无出边，FR-OPS-004）。
var ticketTransitions = map[string][]string{
	"PENDING_DISPATCH": {"DISPATCHED", "CANCELLED"},
	"DISPATCHED":       {"ACCEPTED", "CANCELLED"}, // 超时升级不改状态：只释放受理人 + 顺延 deadline
	"ACCEPTED":         {"PROCESSING", "CANCELLED"},
	"PROCESSING":       {"SUBMITTED", "CANCELLED"},
	"SUBMITTED":        {"VERIFIED", "PROCESSING", "CANCELLED"}, // 验收退回 / 撤回
	"VERIFIED":         {"CLOSED"},
	"CLOSED":           {},
	"CANCELLED":        {},
}

func transitionAllowed(from, to string) bool {
	for _, t := range ticketTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// createTicketCore 创建工单（RPC 与巡检/告警转单共用；client_action_id 幂等，FR-OPS-008）。
func createTicketCore(ctx context.Context, sc *svc.ServiceContext, tid int64, in *model.Ticket) (*model.Ticket, error) {
	if !ticketTypes[in.Type] {
		return nil, errTicketTypeBad
	}
	// 离线重放幂等：同 client_action_id 直接查回
	if in.ClientActionId.Valid && in.ClientActionId.String != "" {
		if dup, err := sc.Models.Ticket.FindByClientActionScoped(ctx, tid, in.ClientActionId.String); err == nil {
			return dup, nil
		} else if err != model.ErrNotFound {
			return nil, err
		}
	}

	// station_name 冗余补全
	if in.StationId > 0 && in.StationName == "" && sc.Station != nil {
		if resp, err := sc.Station.GetStation(ctx, &stationclient.GetStationReq{StationId: in.StationId}); err == nil && resp.Station != nil {
			in.StationName = resp.Station.Name
		}
	}

	if in.Priority == "" {
		in.Priority = "P2"
	}
	in.Status = "PENDING_DISPATCH"
	in.TenantId = tid
	in.TicketNo = "TK" + time.Now().Format("20060102") + fmt.Sprintf("%06d", sc.Snowflake.MustNextID()%1000000)
	err := sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := sc.Models.Ticket.InsertTx(ctx, session, in); err != nil {
			return err
		}
		return sc.Models.TicketLog.InsertTx(ctx, session, &model.TicketLog{
			LogId:       sc.Snowflake.MustNextID(),
			TicketId:    in.TicketId,
			Action:      "create",
			ToStatus:    "PENDING_DISPATCH",
			OperatorUid: in.CreatedBy.Int64,
			Note:        in.Content,
			TenantId:    tid,
		})
	})
	if err != nil {
		return nil, err
	}
	return in, nil
}

// matchStaffCore 派单候选：party.ListStaff 分页拉取 → 区域+技能本地过滤
// （FR-OPS-004/FR-PTY-005）。区域口径：场站省份与 work_region 互为前缀；
// 技能口径：required_skills ⊆ skill_tags。
func matchStaffCore(ctx context.Context, sc *svc.ServiceContext, tid, stationId int64, requiredSkills []string) ([]*partyclient.StaffItem, error) {
	if sc.Party == nil {
		return nil, errTicketNoStaff
	}
	// 场站省份（区域匹配锚点；无场站 = 不限区域）
	var province string
	if stationId > 0 && sc.Station != nil {
		if resp, err := sc.Station.GetStation(ctx, &stationclient.GetStationReq{StationId: stationId}); err == nil && resp.Station != nil {
			province = resp.Station.Province
		}
	}

	var list []*partyclient.StaffItem
	for page := int64(1); ; page++ {
		resp, err := sc.Party.ListStaff(ctx, &partyclient.ListStaffReq{Page: page, Size: 100})
		if err != nil {
			return nil, errcode.ErrServiceUnavailable.WithMsg("派单候选拉取失败").WithCause(err)
		}
		for _, s := range resp.List {
			if s.UserId <= 0 {
				continue // 未开账号无法接单
			}
			if province != "" && s.WorkRegion != "" &&
				!strings.HasPrefix(province, s.WorkRegion) && !strings.HasPrefix(s.WorkRegion, province) {
				continue
			}
			if len(requiredSkills) > 0 && !skillsCover(s.SkillTags, requiredSkills) {
				continue
			}
			list = append(list, s)
		}
		if int64(len(resp.List)) < 100 || page*100 >= resp.Total {
			break
		}
	}
	return list, nil
}

// skillsCover staff.skill_tags(JSON 数组) 覆盖全部 required。
func skillsCover(skillTagsJSON string, required []string) bool {
	if skillTagsJSON == "" {
		return false
	}
	var tags []string
	if err := json.Unmarshal([]byte(skillTagsJSON), &tags); err != nil {
		return false
	}
	set := make(map[string]bool, len(tags))
	for _, t := range tags {
		set[t] = true
	}
	for _, r := range required {
		if !set[r] {
			return false
		}
	}
	return true
}

// dispatchTicketCore 派单（PENDING_DISPATCH→DISPATCHED；uid=0 时自动取第一候选；
// accept_deadline 写入 = 接单超时扫描锚点，ADR-09）。SLA 计时器随后挂接（容错）。
func dispatchTicketCore(ctx context.Context, sc *svc.ServiceContext, tid, ticketId, assigneeUid int64, acceptTimeoutMin int) (int64, string, time.Time, error) {
	tk, err := sc.Models.Ticket.FindOneScoped(ctx, tid, ticketId)
	if err != nil {
		return 0, "", time.Time{}, errTicketNotFound
	}
	if tk.Status != "PENDING_DISPATCH" {
		return 0, "", time.Time{}, errTicketStatusBad
	}

	name := ""
	if assigneeUid == 0 { // 自动派单：第一候选
		cands, merr := matchStaffCore(ctx, sc, tid, tk.StationId, nil)
		if merr != nil {
			return 0, "", time.Time{}, merr
		}
		if len(cands) == 0 {
			return 0, "", time.Time{}, errTicketNoStaff
		}
		assigneeUid = cands[0].UserId
		name = cands[0].Name
	}

	now := time.Now()
	deadline := now.Add(time.Duration(acceptTimeoutMin) * time.Minute)
	err = sc.Conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		ok, err := sc.Models.Ticket.UpdateDispatchTx(ctx, session, tid, ticketId, assigneeUid, name, now, deadline)
		if err != nil {
			return err
		}
		if !ok {
			return errTicketStatusBad
		}
		return sc.Models.TicketLog.InsertTx(ctx, session, &model.TicketLog{
			LogId: sc.Snowflake.MustNextID(), TicketId: ticketId,
			Action: "dispatch", FromStatus: "PENDING_DISPATCH", ToStatus: "DISPATCHED",
			OperatorUid: ctxkit.UID(ctx),
			Note:        "派单 → " + name + "，接单截止 " + deadline.Format(time.DateTime),
			TenantId:    tid,
		})
	})
	if err != nil {
		return 0, "", time.Time{}, err
	}
	// SLA 计时器挂接（独立失败不影响派单；策略来自合同绑定快照，FR-CTR-007）
	if err := attachSlaTimer(ctx, sc, tid, tk, now); err != nil {
		logx.Errorf("ops: SLA 计时器挂接失败 ticket=%s: %v", tk.TicketNo, err)
	}
	// 派单通知（工单派单模板，FR-NTF-003 场景之一；Outbox 最终一致）
	notify(sc, tid, assigneeUid, "ticket_dispatched", "TICKET",
		"ticket:"+tk.TicketNo, map[string]string{"ticket_no": tk.TicketNo, "title": tk.Title})
	return assigneeUid, name, deadline, nil
}

// attachSlaTimer 场站 → order_no → contract SLA 绑定快照建计时器。
// 无绑定/无 RPC = 不计时（工单照常流转）。
func attachSlaTimer(ctx context.Context, sc *svc.ServiceContext, tid int64, tk *model.Ticket, dispatchAt time.Time) error {
	if sc.Contract == nil || tk.StationId == 0 {
		return nil
	}
	// 已挂过（重派/重放幂等）
	if _, err := sc.Models.SlaTimer.FindOneByTicket(ctx, tid, tk.TicketId); err == nil {
		return nil
	}
	if tk.StationName == "" || sc.Station == nil {
		return nil
	}
	st, err := sc.Station.GetStation(ctx, &stationclient.GetStationReq{StationId: tk.StationId})
	if err != nil || st == nil || st.Station == nil || st.Station.OrderNo == "" {
		return nil
	}
	bind, err := sc.Contract.GetContractSlaByOrderNo(ctx, &contractclient.GetContractSlaByOrderNoReq{OrderNo: st.Station.OrderNo})
	if err != nil || bind == nil || bind.Strategy == nil {
		return nil // 未绑定：不计时
	}
	snap, _ := json.Marshal(map[string]any{
		"strategy_id": bind.Strategy.StrategyId, "code": bind.Strategy.Code,
		"name": bind.Strategy.Name, "level": bind.Strategy.Level,
		"response_minutes": bind.Strategy.ResponseMinutes, "resolve_minutes": bind.Strategy.ResolveMinutes,
	})
	respDue := dispatchAt.Add(time.Duration(bind.Strategy.ResponseMinutes) * time.Minute)
	resoDue := dispatchAt.Add(time.Duration(bind.Strategy.ResolveMinutes) * time.Minute)
	return sc.Models.SlaTimer.InsertTx(ctx, sc.Conn, &model.SlaTimer{
		TimerId:          sc.Snowflake.MustNextID(),
		TicketId:         tk.TicketId,
		StrategyId:       bind.Strategy.StrategyId,
		StrategySnapshot: string(snap),
		ResponseDueAt:    respDue,
		ResolveDueAt:     resoDue,
		TenantId:         tid,
	})
}

// escalateTicket 接单超时升级（cron 扫描调用；ADR-09：状态不变，释放受理人 +
// 升级计数+1 + deadline 顺延；升级后可重新派单）。
func escalateTicket(ctx context.Context, sc *svc.ServiceContext, tk *model.Ticket, acceptTimeoutMin int) error {
	newDeadline := time.Now().Add(time.Duration(acceptTimeoutMin) * time.Minute)
	if err := sc.Models.Ticket.UpdateEscalate(ctx, tk.TenantId, tk.TicketId, newDeadline); err != nil {
		return err
	}
	return sc.Models.TicketLog.InsertTx(ctx, sc.Conn, &model.TicketLog{
		LogId: sc.Snowflake.MustNextID(), TicketId: tk.TicketId,
		Action: "escalate", FromStatus: "DISPATCHED", ToStatus: "DISPATCHED",
		OperatorUid: 0,
		Note: fmt.Sprintf("接单超时升级（第 %d 次），受理人已释放，截止顺延至 %s",
			tk.EscalateLevel+1, newDeadline.Format(time.DateTime)),
		TenantId: tk.TenantId,
	})
}
