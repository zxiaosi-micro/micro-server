package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTicketLogic {
	return &GetTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTicketLogic) GetTicket(in *pb.GetTicketReq) (*pb.GetTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errTicketNotFound
	}
	logs, err := l.svcCtx.Models.TicketLog.ListByTicket(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetTicketResp{Ticket: pbTicket(tk)}
	for _, g := range logs {
		resp.Logs = append(resp.Logs, pbTicketLog(g))
	}
	return resp, nil
}
