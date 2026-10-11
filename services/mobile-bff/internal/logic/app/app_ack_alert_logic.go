// app_ack_alert_logic.go · 告警确认。

package app

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppAckAlertLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppAckAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppAckAlertLogic {
	return &AppAckAlertLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppAckAlertLogic) AppAckAlert(req *types.AppAckAlertReq) (resp *types.SimpleResp, err error) {
	if _, err := l.svcCtx.Ops.AckAlert(l.ctx, &opsb.AckAlertReq{AlertId: parseI64(req.ID), Note: req.Note}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
