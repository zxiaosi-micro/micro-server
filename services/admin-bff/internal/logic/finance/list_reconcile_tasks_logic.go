// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package finance

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListReconcileTasksLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListReconcileTasksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListReconcileTasksLogic {
	return &ListReconcileTasksLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListReconcileTasksLogic) ListReconcileTasks(req *types.ReconcileListReq) (resp *types.ReconcileListResp, err error) {
	respOut, err := l.svcCtx.Finance.ListReconcileTask(l.ctx, &finpb.ListReconcileTaskReq{
		Status: req.Status, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.ReconcileListResp{Total: int(respOut.Total)}
	for _, t := range respOut.List {
		out.List = append(out.List, types.ReconcileTaskView{
			TaskId: strconv.FormatInt(t.TaskId, 10), TaskNo: t.TaskNo, Type: t.Type,
			BizDate: t.BizDate, DiffReport: t.DiffReport, Status: t.Status,
			Resolution: t.Resolution, ResolveRemark: t.ResolveRemark,
			ResolvedBy: strconv.FormatInt(t.ResolvedBy, 10), CreatedAt: t.CreatedAt,
		})
	}
	return out, nil
}
