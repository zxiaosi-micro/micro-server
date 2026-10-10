package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ArchiveAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveAlertLogic {
	return &ArchiveAlertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ArchiveAlert 归档（终态，只读留痕）。
func (l *ArchiveAlertLogic) ArchiveAlert(in *pb.ArchiveAlertReq) (*pb.ArchiveAlertResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Models.Alert.UpdateArchive(l.ctx, tid, in.AlertId); err != nil {
		return nil, err
	}
	return &pb.ArchiveAlertResp{}, nil
}
