package logic

import (
	"context"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListExportTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListExportTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListExportTaskLogic {
	return &ListExportTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListExportTask 导出任务列表（下载中心；page/size 有界）。
func (l *ListExportTaskLogic) ListExportTask(in *pb.ListExportTaskReq) (*pb.ListExportTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := int(in.Page), int(in.Size)
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	rows, total, err := l.svcCtx.Models.Export.List(l.ctx, tid, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListExportTaskResp{Total: total}
	for _, t := range rows {
		resp.List = append(resp.List, exportTaskItem(t))
	}
	return resp, nil
}
