// mappers.go · pb → types 视图映射（customer 包）。

package customer

import (
	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/types"
	npb "micro-server/services/notification/pb"
	opsb "micro-server/services/ops/pb"
	spb "micro-server/services/station/pb"
)

func pbDevice(d *dpb.DeviceView) types.CustomerDeviceItem {
	if d == nil {
		return types.CustomerDeviceItem{}
	}
	return types.CustomerDeviceItem{
		DeviceId:    i64s(d.DeviceId),
		Sn:          d.Sn,
		ProductKey:  d.ProductKey,
		Model:       d.Model,
		Status:      d.Status,
		ActivatedAt: d.ActivatedAt,
	}
}

func pbShadow(s *dpb.GetDeviceShadowResp) types.ShadowView {
	if s == nil {
		return types.ShadowView{Metrics: []types.MetricView{}}
	}
	metrics := []types.MetricView{}
	for _, m := range s.Metrics {
		metrics = append(metrics, types.MetricView{Key: m.Key, Value: m.Value})
	}
	return types.ShadowView{Ts: i64s(s.Ts), Metrics: metrics}
}

func pbWarranty(w *ctpb.WarrantyItem) types.WarrantyView {
	if w == nil {
		return types.WarrantyView{}
	}
	return types.WarrantyView{
		WarrantyId: i64s(w.WarrantyId),
		WarrantyNo: w.WarrantyNo,
		Level:      w.Level,
		TargetKey:  w.TargetKey,
		Status:     w.Status,
		StartAt:    w.StartAt,
		EndAt:      w.EndAt,
		Months:     int64(w.Months),
	}
}

func pbContract(c *ctpb.ContractDetail) types.ContractView {
	if c == nil {
		return types.ContractView{}
	}
	return types.ContractView{
		ContractId: i64s(c.ContractId),
		ContractNo: c.ContractNo,
		Name:       c.Name,
		Type:       c.Type,
		Status:     c.Status,
		Amount:     c.Amount,
		CreatedAt:  c.CreatedAt,
	}
}

func pbCustomerTicket(t *opsb.TicketItem) types.CustomerTicketItem {
	if t == nil {
		return types.CustomerTicketItem{}
	}
	return types.CustomerTicketItem{
		TicketId:    i64s(t.TicketId),
		TicketNo:    t.TicketNo,
		Type:        t.Type,
		Title:       t.Title,
		Content:     t.Content,
		Status:      t.Status,
		Priority:    t.Priority,
		Sn:          t.Sn,
		StationName: t.StationName,
		CreatedAt:   t.CreatedAt,
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

func pbMessage(m *npb.MessageItem) types.MessageView {
	if m == nil {
		return types.MessageView{}
	}
	return types.MessageView{
		MessageId: i64s(m.MessageId),
		Title:     m.Title,
		Content:   m.Content,
		IsRead:    m.IsRead,
		BizType:   m.BizType,
		BizId:     m.BizId,
		CreatedAt: m.CreatedAt,
	}
}

func pbStation(s *spb.StationView) types.CustomerStationItem {
	if s == nil {
		return types.CustomerStationItem{}
	}
	return types.CustomerStationItem{
		StationId:   i64s(s.StationId),
		StationNo:   s.StationNo,
		Name:        s.Name,
		Type:        s.Type,
		Status:      s.Status,
		Address:     s.Province + s.City + s.Address,
		CapacityKwh: s.CapacityKwh,
		PowerKw:     s.PowerKw,
		OrderNo:     s.OrderNo,
	}
}
