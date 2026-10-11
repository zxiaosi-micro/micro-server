// ticket_detail_logic.go · 工单进度详情（party 归属校验后放行）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type TicketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketDetailLogic {
	return &TicketDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *TicketDetailLogic) TicketDetail(req *types.IDPath) (resp *types.CustomerTicketDetail, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Ops.GetTicket(l.ctx, &opsb.GetTicketReq{TicketId: parseI64(req.ID)})
	if err != nil {
		return nil, err
	}
	if out.GetTicket().GetPartyId() != partyId {
		return nil, errcode.ErrPermissionDenied
	}
	return &types.CustomerTicketDetail{
		Ticket: pbCustomerTicket(out.Ticket),
		Logs:   pbTicketLogs(out.Logs),
	}, nil
}
