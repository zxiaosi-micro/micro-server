// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTemplatesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 模板列表(perm: notification:template:list)
func NewListTemplatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTemplatesLogic {
	return &ListTemplatesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTemplatesLogic) ListTemplates(req *types.TemplateListReq) (resp *types.TemplateListResp, err error) {
	r, err := l.svcCtx.Notification.ListTemplate(l.ctx, &notificationPb.ListTemplateReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.TemplateListResp{Total: r.Total}
	for _, t := range r.List {
		resp.List = append(resp.List, types.TemplateItem{
			TemplateID: strconv.FormatInt(t.TemplateId, 10), Code: t.Code,
			TitleTemplate: t.TitleTemplate, ContentTemplate: t.ContentTemplate,
			Channel: t.Channel, Status: int(t.Status),
		})
	}
	return resp, nil
}
