package logic

import (
	"context"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetUserSettingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserSettingLogic {
	return &GetUserSettingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetUserSettingLogic) GetUserSetting(in *pb.GetUserSettingReq) (*pb.GetUserSettingResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 {
		return nil, errUserRequired
	}
	list, err := l.svcCtx.Models.UserSetting.FindByUser(l.ctx, tid, in.UserId, in.TemplateCode)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.GetUserSettingResp{}
	for _, s := range list {
		resp.List = append(resp.List, &pb.UserSettingItem{
			UserId:       s.UserId,
			TemplateCode: s.TemplateCode,
			Enabled:      s.Enabled == 1,
			QuietHours:   s.QuietHours.String,
		})
	}
	return resp, nil
}
