// app_inspections_logic.go · 我的巡检任务列表。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppInspectionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppInspectionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppInspectionsLogic {
	return &AppInspectionsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppInspectionsLogic) AppInspections(req *types.AppInspectionListReq) (resp *types.AppInspectionListResp, err error) {
	out, err := l.svcCtx.Ops.ListInspectionTask(l.ctx, &opsb.ListInspectionTaskReq{
		Status: req.Status,
		Page:   req.Page,
		Size:   req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.AppInspectionListResp{Total: out.Total, List: []types.AppInspectionTask{}}
	for _, t := range out.List {
		resp.List = append(resp.List, pbInspectionTask(t, nil))
	}
	return resp, nil
}
