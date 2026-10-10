package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PauseSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPauseSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PauseSlaTimerLogic {
	return &PauseSlaTimerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PauseSlaTimer 暂停（必填原因，留痕；pause_minutes 累计）。
func (l *PauseSlaTimerLogic) PauseSlaTimer(in *pb.PauseSlaTimerReq) (*pb.PauseSlaTimerResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := pauseSlaTimer(l.ctx, l.svcCtx, tid, in.TicketId, in.Reason); err != nil {
		return nil, err
	}
	return &pb.PauseSlaTimerResp{}, nil
}
