package logic

import (
	"context"
	"encoding/json"
	"time"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeliverLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeliverLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeliverLogic {
	return &DeliverLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// Deliver 直接投递（管理端手动发通知/测试桩；与事件消费共用内核，口径一致）。
func (l *DeliverLogic) Deliver(in *pb.DeliverReq) (*pb.DeliverResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 || in.TemplateCode == "" {
		return nil, errUserRequired
	}
	params := map[string]string{}
	if in.ParamsJson != "" {
		if err := json.Unmarshal([]byte(in.ParamsJson), &params); err != nil {
			return nil, errcode.ErrBadRequest.WithMsg("params_json 不合法")
		}
	}

	// 免打扰窗口（RPC 侧显式校验并给出语义化原因）
	settings, _ := l.svcCtx.Models.UserSetting.FindByUser(l.ctx, tid, in.UserId, in.TemplateCode)
	for _, s := range settings {
		if s.Enabled == 1 && s.QuietHours.Valid && s.QuietHours.String != "" && inQuietHours(s.QuietHours.String, time.Now()) {
			return &pb.DeliverResp{Delivered: false, Reason: errQuietInEffect.Msg()}, nil
		}
	}

	out := deliver(l.ctx, l.svcCtx, tid, deliverInput{
		UserID:       in.UserId,
		TemplateCode: in.TemplateCode,
		Params:       params,
		BizType:      in.BizType,
		BizID:        in.BizId,
	})
	return &pb.DeliverResp{Delivered: out.Delivered, Reason: out.Reason, MessageId: out.MessageID}, nil
}
