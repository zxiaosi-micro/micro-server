package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
	"time"
)

type AcceptTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcceptTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptTicketLogic {
	return &AcceptTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AcceptTicketLogic) AcceptTicket(in *pb.AcceptTicketReq) (*pb.AcceptTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errTicketNotFound
	}
	if !transitionAllowed(tk.Status, "ACCEPTED") {
		return nil, errTicketStatusBad
	}
	ok, err := l.svcCtx.Models.Ticket.CasStatus(l.ctx, tid, in.TicketId, "DISPATCHED", "ACCEPTED")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errTicketStatusBad
	}
	now := time.Now()
	_ = l.svcCtx.Models.Ticket.UpdateCloseFields(l.ctx, tid, in.TicketId, map[string]any{"accepted_at": now})
	// SLA：接单即响应达成
	_ = l.svcCtx.Models.SlaTimer.MarkResponded(l.ctx, tid, in.TicketId, now)
	_ = l.svcCtx.Models.TicketLog.InsertTx(l.ctx, l.svcCtx.Conn, &model.TicketLog{
		LogId: l.svcCtx.Snowflake.MustNextID(), TicketId: in.TicketId,
		Action: "accept", FromStatus: "DISPATCHED", ToStatus: "ACCEPTED",
		OperatorUid: uid, TenantId: tid,
	})
	return &pb.AcceptTicketResp{}, nil
}
