// app_inspection_detail_logic.go · 巡检任务详情（含明细项）。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppInspectionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppInspectionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppInspectionDetailLogic {
	return &AppInspectionDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppInspectionDetailLogic) AppInspectionDetail(req *types.IDPath) (resp *types.AppInspectionTask, err error) {
	out, err := l.svcCtx.Ops.GetInspectionTask(l.ctx, &opsb.GetInspectionTaskReq{TaskId: parseI64(req.ID)})
	if err != nil {
		return nil, err
	}
	task := pbInspectionTask(out.Task, out.Items)
	return &task, nil
}
