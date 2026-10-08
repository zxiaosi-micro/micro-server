// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type UpsertNotifySettingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 通知设置 Upsert(perm: notification:setting:update)
func NewUpsertNotifySettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertNotifySettingLogic {
	return &UpsertNotifySettingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertNotifySettingLogic) UpsertNotifySetting(req *types.NotifySettingUpsertReq) (resp *types.SimpleResp, err error) {
	uid := parseID(req.UserID)
	if uid == 0 {
		uid = ctxkit.UID(l.ctx)
	}
	_, err = l.svcCtx.Notification.UpdateUserSetting(l.ctx, &notificationPb.UpdateUserSettingReq{
		UserId: uid, TemplateCode: req.TemplateCode, Enabled: req.Enabled, QuietHours: req.QuietHours,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
