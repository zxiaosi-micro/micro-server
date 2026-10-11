package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketLogic {
	return &ListTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTicketLogic) ListTicket(in *pb.ListTicketReq) (*pb.ListTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Ticket.ListPage(l.ctx, tid, in.Status, in.Type, in.StationId, in.AssigneeUid, in.PartyId, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListTicketResp{Total: total}
	for _, t := range list {
		resp.List = append(resp.List, pbTicket(t))
	}
	return resp, nil
}
