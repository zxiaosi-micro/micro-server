package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListInspectionTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInspectionTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInspectionTemplateLogic {
	return &ListInspectionTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListInspectionTemplateLogic) ListInspectionTemplate(in *pb.ListInspectionTemplateReq) (*pb.ListInspectionTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.InsTmpl.ListPage(l.ctx, tid, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListInspectionTemplateResp{Total: total}
	for _, t := range list {
		resp.List = append(resp.List, &pb.InspectionTemplateView{
			TemplateId: t.TemplateId, Name: t.Name,
			ItemsSchema: t.ItemsSchema, CreatedAt: t.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
