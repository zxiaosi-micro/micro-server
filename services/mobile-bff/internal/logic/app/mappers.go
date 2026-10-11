// mappers.go · pb → types 视图映射（app 包）。

package app

import (
	"encoding/json"
	"strings"

	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"
)

func pbTicket(t *opsb.TicketItem) types.AppTicketItem {
	if t == nil {
		return types.AppTicketItem{}
	}
	return types.AppTicketItem{
		TicketId:       i64s(t.TicketId),
		TicketNo:       t.TicketNo,
		Type:           t.Type,
		Title:          t.Title,
		Content:        t.Content,
		Priority:       t.Priority,
		StationId:      i64s(t.StationId),
		StationName:    t.StationName,
		DeviceId:       i64s(t.DeviceId),
		Sn:             t.Sn,
		AssigneeUid:    i64s(t.AssigneeUid),
		AssigneeName:   t.AssigneeName,
		Status:         t.Status,
		EscalateLevel:  int64(t.EscalateLevel),
		AcceptDeadline: t.AcceptDeadline,
		CreatedAt:      t.CreatedAt,
	}
}

func pbTicketLogs(logs []*opsb.TicketLogItem) []types.AppTicketLogItem {
	out := make([]types.AppTicketLogItem, 0, len(logs))
	for _, g := range logs {
		out = append(out, types.AppTicketLogItem{
			LogId:       i64s(g.LogId),
			Action:      g.Action,
			FromStatus:  g.FromStatus,
			ToStatus:    g.ToStatus,
			OperatorUid: i64s(g.OperatorUid),
			Note:        g.Note,
			CreatedAt:   g.CreatedAt,
		})
	}
	return out
}

func pbTicketBrief(t *opsb.TicketBrief) types.AppTicketBrief {
	if t == nil {
		return types.AppTicketBrief{}
	}
	return types.AppTicketBrief{
		TicketId:      i64s(t.TicketId),
		TicketNo:      t.TicketNo,
		Type:          t.Type,
		Title:         t.Title,
		Priority:      t.Priority,
		StationId:     i64s(t.StationId),
		StationName:   t.StationName,
		Status:        t.Status,
		EscalateLevel: int64(t.EscalateLevel),
		CreatedAt:     t.CreatedAt,
	}
}

func pbAlertBrief(a *opsb.AlertBrief) types.AppAlertBrief {
	if a == nil {
		return types.AppAlertBrief{}
	}
	return types.AppAlertBrief{
		AlertId:        i64s(a.AlertId),
		Level:          a.Level,
		Source:         a.Source,
		Metric:         a.Metric,
		Title:          a.Title,
		StationId:      i64s(a.StationId),
		StationName:    a.StationName,
		AggregateCount: int64(a.AggregateCount),
		Status:         a.Status,
		FirstSeenAt:    a.FirstSeenAt,
	}
}

func pbInspectionTask(t *opsb.InspectionTaskView, items []*opsb.InspectionItemView) types.AppInspectionTask {
	if t == nil {
		return types.AppInspectionTask{Items: []types.AppInspectionItem{}}
	}
	tk := types.AppInspectionTask{
		TaskId:       i64s(t.TaskId),
		StationId:    i64s(t.StationId),
		StationName:  t.StationName,
		Title:        t.Title,
		ScheduledFor: t.ScheduledFor,
		Status:       t.Status,
		AnomalyCount: int64(t.AnomalyCount),
		SubmittedAt:  t.SubmittedAt,
		Items:        []types.AppInspectionItem{},
	}
	for _, it := range items {
		tk.Items = append(tk.Items, types.AppInspectionItem{
			ItemId:    i64s(it.ItemId),
			Name:      it.Name,
			Result:    it.Result,
			Anomalous: it.Anomalous,
			TicketId:  i64s(it.TicketId),
		})
	}
	return tk
}

// jsonStrings JSON 数组串 → []string（空串安全）。
func jsonStrings(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}
