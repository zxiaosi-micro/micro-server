// app_accept_ticket_logic.go · 接单。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppAcceptTicketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppAcceptTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppAcceptTicketLogic {
	return &AppAcceptTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppAcceptTicketLogic) AppAcceptTicket(req *types.IDPath) (resp *types.SimpleResp, err error) {
	if _, err := l.svcCtx.Ops.AcceptTicket(l.ctx, &opsb.AcceptTicketReq{TicketId: parseI64(req.ID)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
