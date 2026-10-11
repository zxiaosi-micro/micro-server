// app_pending_tickets_logic.go · 工单待办增量（FR-OPS-010 轮询；assignee=我）。

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

type AppPendingTicketsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppPendingTicketsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppPendingTicketsLogic {
	return &AppPendingTicketsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppPendingTicketsLogic) AppPendingTickets(req *types.AppPendingTicketsReq) (resp *types.AppPendingTicketsResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out, err := l.svcCtx.Ops.PendingTickets(l.ctx, &opsb.PendingTicketsReq{
		AssigneeUid: uid,
		SinceId:     parseI64(req.SinceId),
		Limit:       20,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AppPendingTicketsResp{UnreadCount: out.UnreadCount, MaxId: i64s(out.MaxId), Items: []types.AppTicketBrief{}}
	for _, t := range out.Items {
		resp.Items = append(resp.Items, pbTicketBrief(t))
	}
	return resp, nil
}
