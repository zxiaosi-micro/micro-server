// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertTemplateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 模板 Upsert(perm: notification:template:update)
func NewUpsertTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertTemplateLogic {
	return &UpsertTemplateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertTemplateLogic) UpsertTemplate(req *types.TemplateUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Notification.UpsertTemplate(l.ctx, &notificationPb.UpsertTemplateReq{
		Code: req.Code, TitleTemplate: req.TitleTemplate, ContentTemplate: req.ContentTemplate,
		Channel: req.Channel, Status: int32(req.Status),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
