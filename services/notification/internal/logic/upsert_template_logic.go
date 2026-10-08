package logic

import (
	"context"

	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/notification/internal/model"
	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpsertTemplateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertTemplateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertTemplateLogic {
	return &UpsertTemplateLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpsertTemplate 模板幂等写入（code UK；存在即更新）。
func (l *UpsertTemplateLogic) UpsertTemplate(in *pb.UpsertTemplateReq) (*pb.UpsertTemplateResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Code == "" || in.TitleTemplate == "" || in.ContentTemplate == "" {
		return nil, errTemplateBad
	}
	if in.Channel == "" {
		in.Channel = "INBOX"
	}
	switch in.Channel {
	case "INBOX", "SMS", "EMAIL", "WECHAT":
	default:
		return nil, errcode.ErrBadRequest.WithMsg("channel 不合法")
	}
	status := int64(in.Status)
	if status == 0 {
		status = 1
	}

	existing, err := l.svcCtx.Models.Template.FindByCode(l.ctx, tid, in.Code)
	if err != nil && err != model.ErrNotFound {
		return nil, errcode.Internal.WithCause(err)
	}
	if existing != nil {
		if err := l.svcCtx.Models.Template.UpdateColumns(l.ctx, tid, existing.TemplateId,
			in.TitleTemplate, in.ContentTemplate, in.Channel, status, ctxkit.UID(l.ctx)); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		return &pb.UpsertTemplateResp{}, nil
	}

	_, err = l.svcCtx.Models.Template.Insert(l.ctx, &model.NotifyTemplate{
		TemplateId:      l.svcCtx.Snowflake.MustNextID(),
		Code:            in.Code,
		TitleTemplate:   in.TitleTemplate,
		ContentTemplate: in.ContentTemplate,
		Channel:         in.Channel,
		Status:          status,
		TenantId:        tid,
	})
	if err != nil {
		if isDupKey(err) {
			return nil, errTemplateUsed
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpsertTemplateResp{}, nil
}
