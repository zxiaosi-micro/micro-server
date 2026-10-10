package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DispatchTicketLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDispatchTicketLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DispatchTicketLogic {
	return &DispatchTicketLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DispatchTicket 派单（assignee_uid 空 = MatchStaff 自动取第一候选）。
func (l *DispatchTicketLogic) DispatchTicket(in *pb.DispatchTicketReq) (*pb.DispatchTicketResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	timeoutMin := int(in.AcceptTimeoutMin)
	if timeoutMin <= 0 {
		timeoutMin = acceptTimeoutMin(l.svcCtx)
	}
	uid, name, deadline, err := dispatchTicketCore(l.ctx, l.svcCtx, tid, in.TicketId, in.AssigneeUid, timeoutMin)
	if err != nil {
		return nil, err
	}
	return &pb.DispatchTicketResp{AssigneeUid: uid, AssigneeName: name, AcceptDeadline: deadline.UnixMilli()}, nil
}
