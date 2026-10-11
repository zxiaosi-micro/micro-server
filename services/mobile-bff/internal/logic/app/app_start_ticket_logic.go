// app_start_ticket_logic.go · 开工。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppStartTicketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppStartTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppStartTicketLogic {
	return &AppStartTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppStartTicketLogic) AppStartTicket(req *types.IDPath) (resp *types.SimpleResp, err error) {
	if _, err := l.svcCtx.Ops.StartTicket(l.ctx, &opsb.StartTicketReq{TicketId: parseI64(req.ID)}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
