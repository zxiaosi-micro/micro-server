// app_submit_ticket_logic.go · 提交处理结果（client_action_id 幂等——离线草稿恢复同步可安全重试）。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppSubmitTicketLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppSubmitTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppSubmitTicketLogic {
	return &AppSubmitTicketLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppSubmitTicketLogic) AppSubmitTicket(req *types.AppSubmitTicketReq) (resp *types.SimpleResp, err error) {
	if _, err := l.svcCtx.Ops.SubmitTicket(l.ctx, &opsb.SubmitTicketReq{
		TicketId:       parseI64(req.ID),
		Result:         req.Result,
		ClientActionId: req.ClientActionId,
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
