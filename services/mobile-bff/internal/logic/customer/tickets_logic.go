// tickets_logic.go · 我的工单（ops.ListTicket party_id 过滤，S9-01 增量契约）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TicketsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketsLogic {
	return &TicketsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *TicketsLogic) Tickets(req *types.CustomerTicketListReq) (resp *types.CustomerTicketListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Ops.ListTicket(l.ctx, &opsb.ListTicketReq{
		Status:  req.Status,
		PartyId: partyId,
		Page:    req.Page,
		Size:    req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.CustomerTicketListResp{Total: out.Total, List: []types.CustomerTicketItem{}}
	for _, t := range out.List {
		resp.List = append(resp.List, pbCustomerTicket(t))
	}
	return resp, nil
}
