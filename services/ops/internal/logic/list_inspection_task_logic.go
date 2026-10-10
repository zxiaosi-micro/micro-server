package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInspectionTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionTaskLogic {
	return &ListInspectionTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInspectionTaskLogic) ListInspectionTask(in *pb.ListInspectionTaskReq) (*pb.ListInspectionTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.InsTask.ListPage(l.ctx, tid, in.StationId, in.Status, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListInspectionTaskResp{Total: total}
	for _, t := range list {
		resp.List = append(resp.List, pbTask(t))
	}
	return resp, nil
}
