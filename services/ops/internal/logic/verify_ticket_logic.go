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

type VerifyTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyTicketLogic {
	return &VerifyTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// VerifyTicket 验收（pass=false 退回 PROCESSING；告警工单验收后可 CloseAlert）。
func (l *VerifyTicketLogic) VerifyTicket(in *pb.VerifyTicketReq) (*pb.VerifyTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errTicketNotFound
	}
	if !transitionAllowed(tk.Status, "VERIFIED") && !transitionAllowed(tk.Status, "PROCESSING") {
		return nil, errTicketStatusBad
	}
	now := time.Now()
	if in.Pass {
		ok, err := l.svcCtx.Models.Ticket.CasStatus(l.ctx, tid, in.TicketId, "SUBMITTED", "VERIFIED")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errTicketStatusBad
		}
		_ = l.svcCtx.Models.Ticket.UpdateCloseFields(l.ctx, tid, in.TicketId, map[string]any{"verified_at": now})
		// SLA：验收即解决达成
		_ = l.svcCtx.Models.SlaTimer.MarkResolved(l.ctx, tid, in.TicketId, now)
	} else {
		ok, err := l.svcCtx.Models.Ticket.CasStatus(l.ctx, tid, in.TicketId, "SUBMITTED", "PROCESSING")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errTicketStatusBad
		}
	}
	_ = l.svcCtx.Models.TicketLog.InsertTx(l.ctx, l.svcCtx.Conn, &model.TicketLog{
		LogId: l.svcCtx.Snowflake.MustNextID(), TicketId: in.TicketId,
		Action: "verify", FromStatus: "SUBMITTED",
		ToStatus:    map[bool]string{true: "VERIFIED", false: "PROCESSING"}[in.Pass],
		OperatorUid: uid, Note: in.Remark, TenantId: tid,
	})
	return &pb.VerifyTicketResp{}, nil
}
