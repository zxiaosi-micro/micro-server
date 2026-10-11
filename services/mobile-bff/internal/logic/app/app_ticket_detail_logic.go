// app_ticket_detail_logic.go · 工单详情（含流转日志）。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppTicketDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppTicketDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppTicketDetailLogic {
	return &AppTicketDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppTicketDetailLogic) AppTicketDetail(req *types.IDPath) (resp *types.AppTicketDetailResp, err error) {
	out, err := l.svcCtx.Ops.GetTicket(l.ctx, &opsb.GetTicketReq{TicketId: parseI64(req.ID)})
	if err != nil {
		return nil, err
	}
	return &types.AppTicketDetailResp{
		Ticket: pbTicket(out.Ticket),
		Logs:   pbTicketLogs(out.Logs),
	}, nil
}
