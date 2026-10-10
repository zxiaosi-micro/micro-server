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

type CancelTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelTicketLogic {
	return &CancelTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelTicketLogic) CancelTicket(in *pb.CancelTicketReq) (*pb.CancelTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errTicketNotFound
	}
	if !transitionAllowed(tk.Status, "CANCELLED") {
		return nil, errTicketStatusBad
	}
	ok, err := l.svcCtx.Models.Ticket.CasStatus(l.ctx, tid, in.TicketId, tk.Status, "CANCELLED")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errTicketStatusBad
	}
	_ = l.svcCtx.Models.Ticket.UpdateCloseFields(l.ctx, tid, in.TicketId, map[string]any{
		"cancel_reason": in.Reason, "closed_at": time.Now(),
	})
	_ = l.svcCtx.Models.TicketLog.InsertTx(l.ctx, l.svcCtx.Conn, &model.TicketLog{
		LogId: l.svcCtx.Snowflake.MustNextID(), TicketId: in.TicketId,
		Action: "cancel", FromStatus: tk.Status, ToStatus: "CANCELLED",
		OperatorUid: uid, Note: in.Reason, TenantId: tid,
	})
	return &pb.CancelTicketResp{}, nil
}
