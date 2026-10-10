package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSlaTimerLogic {
	return &GetSlaTimerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetSlaTimer 工单计时器详情（策略快照/暂停留痕/分级升级状态）。
func (l *GetSlaTimerLogic) GetSlaTimer(in *pb.GetSlaTimerReq) (*pb.GetSlaTimerResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	timer, err := l.svcCtx.Models.SlaTimer.FindOneByTicket(l.ctx, tid, in.TicketId)
	if err != nil {
		return nil, errSlaNotFound
	}
	return &pb.GetSlaTimerResp{Timer: pbSlaTimer(timer)}, nil
}
