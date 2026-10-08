package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateReconcileTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateReconcileTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReconcileTaskLogic {
	return &CreateReconcileTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateReconcileTaskLogic) CreateReconcileTask(in *pb.CreateReconcileTaskReq) (*pb.CreateReconcileTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	taskId, err := CreateReconcileTaskInternal(l.ctx, l.svcCtx, tid, in.Type, in.BizDate, in.DiffReport)
	if err != nil {
		return nil, err
	}
	t, err := l.svcCtx.Models.ReconcileTask.FindOneScoped(l.ctx, tid, taskId)
	if err != nil {
		return nil, err
	}
	return &pb.CreateReconcileTaskResp{TaskId: taskId, TaskNo: t.TaskNo}, nil
}
