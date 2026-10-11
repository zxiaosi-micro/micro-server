// activate_device_logic.go · 扫码激活（requireParty：激活即绑定本人 party，质保起算事件）。

package customer

import (
	"context"

	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ActivateDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewActivateDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActivateDeviceLogic {
	return &ActivateDeviceLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ActivateDeviceLogic) ActivateDevice(req *types.ActivateDeviceReq) (resp *types.SimpleResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Device.Activate(l.ctx, &dpb.ActivateReq{Sn: req.SN, PartyId: partyId}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
