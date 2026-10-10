package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResumeSlaTimerLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResumeSlaTimerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResumeSlaTimerLogic {
	return &ResumeSlaTimerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResumeSlaTimer 恢复（due_at 顺延累计暂停时长）。
func (l *ResumeSlaTimerLogic) ResumeSlaTimer(in *pb.ResumeSlaTimerReq) (*pb.ResumeSlaTimerResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := resumeSlaTimer(l.ctx, l.svcCtx, tid, in.TicketId); err != nil {
		return nil, err
	}
	return &pb.ResumeSlaTimerResp{}, nil
}
