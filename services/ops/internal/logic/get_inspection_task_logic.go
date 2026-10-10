package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetInspectionTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInspectionTaskLogic {
	return &GetInspectionTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetInspectionTaskLogic) GetInspectionTask(in *pb.GetInspectionTaskReq) (*pb.GetInspectionTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	task, err := l.svcCtx.Models.InsTask.FindOneScoped(l.ctx, tid, in.TaskId)
	if err != nil {
		return nil, errInspectNotFound
	}
	items, err := l.svcCtx.Models.InsItem.ListByTask(l.ctx, tid, in.TaskId)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetInspectionTaskResp{Task: pbTask(task)}
	for _, i := range items {
		resp.Items = append(resp.Items, pbInsItem(i))
	}
	return resp, nil
}
