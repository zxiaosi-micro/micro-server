package logic

import (
	"context"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RebuildReadModelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRebuildReadModelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RebuildReadModelLogic {
	return &RebuildReadModelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RebuildReadModel 读模型全量重建（台账/聚合/watermark 清空 + 消费组版本号 +1）。
// 重启后新消费组从头回放；保留期外历史用 tools/p4backfill 按业务库补发。
func (l *RebuildReadModelLogic) RebuildReadModel(in *pb.RebuildReadModelReq) (*pb.RebuildReadModelResp, error) {
	v, err := RebuildReadModelCore(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	return &pb.RebuildReadModelResp{RebuildVersion: v}, nil
}
