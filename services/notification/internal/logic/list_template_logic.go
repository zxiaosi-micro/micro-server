package logic

import (
	"context"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTemplateLogic {
	return &ListTemplateLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListTemplateLogic) ListTemplate(in *pb.ListTemplateReq) (*pb.ListTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Template.FindPage(l.ctx, tid, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListTemplateResp{Total: total}
	for _, t := range list {
		resp.List = append(resp.List, &pb.TemplateItem{
			TemplateId:      t.TemplateId,
			Code:            t.Code,
			TitleTemplate:   t.TitleTemplate,
			ContentTemplate: t.ContentTemplate,
			Channel:         t.Channel,
			Status:          int32(t.Status),
		})
	}
	return resp, nil
}
