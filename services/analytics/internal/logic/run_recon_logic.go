package logic

import (
	"context"
	"time"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunReconLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRunReconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunReconLogic {
	return &RunReconLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RunRecon 手动触发日级对账（cron daily_recon 同内核；recon_date 空 = 昨天 T-1）。
func (l *RunReconLogic) RunRecon(in *pb.RunReconReq) (*pb.RunReconResp, error) {
	var day time.Time
	var err error
	if in.ReconDate == "" {
		day = statDateOf(time.Now().AddDate(0, 0, -1))
	} else {
		day, err = parseDay(in.ReconDate)
		if err != nil {
			return nil, errReconDateBad
		}
	}
	diff, err := RunReconCore(l.ctx, l.svcCtx, day)
	if err != nil {
		return nil, err
	}
	return &pb.RunReconResp{DiffTotal: diff}, nil
}
