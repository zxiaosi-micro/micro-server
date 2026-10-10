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

type SubmitTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitTicketLogic {
	return &SubmitTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SubmitTicket 处理完成提交（附结果说明）。
func (l *SubmitTicketLogic) SubmitTicket(in *pb.SubmitTicketReq) (*pb.SubmitTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	tk, err := l.svcCtx.Models.Ticket.FindOneScoped(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errTicketNotFound
	}
	if !transitionAllowed(tk.Status, "SUBMITTED") {
		return nil, errTicketStatusBad
	}
	ok, err := l.svcCtx.Models.Ticket.CasStatus(l.ctx, tid, in.TicketId, "PROCESSING", "SUBMITTED")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errTicketStatusBad
	}
	_ = l.svcCtx.Models.Ticket.UpdateCloseFields(l.ctx, tid, in.TicketId, map[string]any{"submitted_at": time.Now()})
	_ = l.svcCtx.Models.TicketLog.InsertTx(l.ctx, l.svcCtx.Conn, &model.TicketLog{
		LogId: l.svcCtx.Snowflake.MustNextID(), TicketId: in.TicketId,
		Action: "submit", FromStatus: "PROCESSING", ToStatus: "SUBMITTED",
		OperatorUid: uid, Note: in.Result, TenantId: tid,
	})
	return &pb.SubmitTicketResp{}, nil
}
