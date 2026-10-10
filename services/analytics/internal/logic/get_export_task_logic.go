package logic

import (
	"context"
	"strconv"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetExportTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExportTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExportTaskLogic {
	return &GetExportTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetExportTask 导出任务查询（下载中心）。
func (l *GetExportTaskLogic) GetExportTask(in *pb.GetExportTaskReq) (*pb.GetExportTaskResp, error) {
	tid, terr := mustTenant(l.ctx)
	if terr != nil {
		return nil, terr
	}
	id, err := strconv.ParseInt(in.ExportId, 10, 64)
	if err != nil || id == 0 {
		return nil, errExportNotFound
	}
	t, err := l.svcCtx.Models.Export.FindOneScoped(l.ctx, id, tid)
	if err != nil {
		return nil, errExportNotFound
	}
	return &pb.GetExportTaskResp{Task: exportTaskItem(t)}, nil
}
