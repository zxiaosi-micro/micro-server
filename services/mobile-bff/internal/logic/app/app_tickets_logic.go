// app_tickets_logic.go · 我的工单列表（assignee=我）。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type AppTicketsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppTicketsLogic {
	return &AppTicketsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppTicketsLogic) AppTickets(req *types.AppTicketListReq) (resp *types.AppTicketListResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out, err := l.svcCtx.Ops.ListTicket(l.ctx, &opsb.ListTicketReq{
		Status:      req.Status,
		AssigneeUid: uid,
		Page:        req.Page,
		Size:        req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AppTicketListResp{Total: out.Total, List: []types.AppTicketItem{}}
	for _, t := range out.List {
		resp.List = append(resp.List, pbTicket(t))
	}
	return resp, nil
}
