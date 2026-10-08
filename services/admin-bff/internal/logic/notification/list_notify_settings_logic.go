// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ListNotifySettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 我的通知设置(perm: notification:setting:list)
func NewListNotifySettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNotifySettingsLogic {
	return &ListNotifySettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListNotifySettingsLogic) ListNotifySettings() (resp *types.NotifySettingListResp, err error) {
	r, err := l.svcCtx.Notification.GetUserSetting(l.ctx, &notificationPb.GetUserSettingReq{UserId: ctxkit.UID(l.ctx)})
	if err != nil {
		return nil, err
	}
	resp = &types.NotifySettingListResp{}
	for _, s := range r.List {
		resp.List = append(resp.List, types.NotifySettingItem{
			UserID: strconv.FormatInt(s.UserId, 10), TemplateCode: s.TemplateCode,
			Enabled: s.Enabled, QuietHours: s.QuietHours,
		})
	}
	return resp, nil
}
