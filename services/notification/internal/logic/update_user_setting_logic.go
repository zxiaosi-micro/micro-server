package logic

import (
	"context"
	"encoding/json"

	"micro-server/services/notification/internal/model"
	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateUserSettingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserSettingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserSettingLogic {
	return &UpdateUserSettingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpdateUserSetting 幂等写入用户通知设置（免打扰/静默；FR-NTF-002/004）。
func (l *UpdateUserSettingLogic) UpdateUserSetting(in *pb.UpdateUserSettingReq) (*pb.UpdateUserSettingResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 || in.TemplateCode == "" {
		return nil, errUserRequired
	}
	if !validJSONString(in.QuietHours) {
		return nil, errQuietHoursBad
	}

	op := ctxkit.UID(l.ctx)
	err = l.svcCtx.Models.UserSetting.Upsert(l.ctx, &model.UserNotifySetting{
		SettingId:    l.svcCtx.Snowflake.MustNextID(),
		UserId:       in.UserId,
		TemplateCode: in.TemplateCode,
		Enabled:      boolToInt(in.Enabled),
		QuietHours:   model.ToNullString(in.QuietHours),
		TenantId:     tid,
		CreatedBy:    model.ToNullInt64(op),
		UpdatedBy:    model.ToNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateUserSettingResp{}, nil
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func validJSONString(s string) bool {
	if s == "" {
		return true
	}
	return json.Valid([]byte(s))
}
