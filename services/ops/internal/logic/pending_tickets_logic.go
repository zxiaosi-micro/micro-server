package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PendingTicketsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPendingTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PendingTicketsLogic {
	return &PendingTicketsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PendingTickets 轮询增量接口（与 PendingAlerts 同构，FR-OPS-010）。
func (l *PendingTicketsLogic) PendingTickets(in *pb.PendingTicketsReq) (*pb.PendingTicketsResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	limit := int(in.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	items, err := l.svcCtx.Models.Ticket.ListPending(l.ctx, tid, in.StationId, in.AssigneeUid, in.Status, in.SinceId, limit)
	if err != nil {
		return nil, err
	}
	resp := &pb.PendingTicketsResp{
		UnreadCount: getTicketActive(l.svcCtx, tid, in.StationId),
	}
	for _, t := range items {
		resp.MaxId = t.TicketId
		resp.Items = append(resp.Items, &pb.TicketBrief{
			TicketId: t.TicketId, TicketNo: t.TicketNo, Type: t.Type,
			Title: t.Title, Priority: t.Priority,
			StationId: t.StationId, StationName: t.StationName,
			Status: t.Status, EscalateLevel: int32(t.EscalateLevel),
			CreatedAt: t.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
