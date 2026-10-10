package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInspectionPlanLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionPlanLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionPlanLogic {
	return &ListInspectionPlanLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInspectionPlanLogic) ListInspectionPlan(in *pb.ListInspectionPlanReq) (*pb.ListInspectionPlanResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.InsPlan.ListPage(l.ctx, tid, in.StationId, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListInspectionPlanResp{Total: total}
	for _, p := range list {
		resp.List = append(resp.List, pbPlan(p))
	}
	return resp, nil
}
