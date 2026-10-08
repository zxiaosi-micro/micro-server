package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/confcenter"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RenderTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRenderTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenderTemplateLogic {
	return &RenderTemplateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RenderTemplateLogic) RenderTemplate(in *pb.RenderTemplateReq) (*pb.RenderTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if !confcenter.Current().RenderEnabled {
		return nil, errRenderDisabled
	}
	tpl, err := l.svcCtx.Models.Template.FindOneScoped(l.ctx, tid, in.TemplateId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errTemplateNotFound
		}
		return nil, err
	}
	b64, err := RenderTemplateInternal(l.Logger, tpl, in.VariablesJson)
	if err != nil {
		return nil, err
	}
	return &pb.RenderTemplateResp{PdfBase64: b64}, nil
}
