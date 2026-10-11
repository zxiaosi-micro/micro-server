// device_shadow_logic.go · 设备影子（requireParty 校验后按 SN 读）。

package customer

import (
	"context"

	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeviceShadowLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeviceShadowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeviceShadowLogic {
	return &DeviceShadowLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeviceShadowLogic) DeviceShadow(req *types.IDPath) (resp *types.ShadowView, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	dev, err := ownDeviceByID(l.ctx, l.svcCtx, parseI64(req.ID), partyId)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Device.GetDeviceShadow(l.ctx, &dpb.GetDeviceShadowReq{Sn: dev.Sn})
	if err != nil {
		return nil, err
	}
	shadow := pbShadow(out)
	return &shadow, nil
}
