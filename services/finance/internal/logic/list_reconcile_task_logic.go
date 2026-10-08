package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListReconcileTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListReconcileTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReconcileTaskLogic {
	return &ListReconcileTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListReconcileTask 对账任务列表（OPEN 优先排查）。
func (l *ListReconcileTaskLogic) ListReconcileTask(in *pb.ListReconcileTaskReq) (*pb.ListReconcileTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.ReconcileTask.ListPage(l.ctx, tid, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListReconcileTaskResp{Total: total}
	for _, t := range list {
		resp.List = append(resp.List, buildReconcileTaskDetail(t))
	}
	return resp, nil
}
