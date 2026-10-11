// ticket_sla_logic.go · 工单 SLA 计时（快照冻结只读，FR-CTR-007）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type TicketSlaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTicketSlaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TicketSlaLogic {
	return &TicketSlaLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *TicketSlaLogic) TicketSla(req *types.IDPath) (resp *types.SlaTimerView, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	tk, err := l.svcCtx.Ops.GetTicket(l.ctx, &opsb.GetTicketReq{TicketId: parseI64(req.ID)})
	if err != nil {
		return nil, err
	}
	if tk.GetTicket().GetPartyId() != partyId {
		return nil, errcode.ErrPermissionDenied
	}
	timer, err := l.svcCtx.Ops.GetSlaTimer(l.ctx, &opsb.GetSlaTimerReq{TicketId: parseI64(req.ID)})
	if err != nil || timer.GetTimer() == nil {
		// 无 SLA 绑定的工单返回空视图（前端隐藏 SLA 区块）
		return &types.SlaTimerView{}, nil
	}
	t := timer.Timer
	return &types.SlaTimerView{
		ResponseDueAt:  t.ResponseDueAt,
		ResolveDueAt:   t.ResolveDueAt,
		PausedAt:       t.PausedAt,
		PauseTotalMin:  int64(t.PauseTotalMin),
		EscalateLevel:  int64(t.EscalateLevel),
		RespondedAt:    t.RespondedAt,
		ResolvedAt:     t.ResolvedAt,
		ResponseBreach: t.ResponseBreachAt,
		ResolveBreach:  t.ResolveBreachAt,
		Snapshot:       t.StrategySnapshot,
	}, nil
}
