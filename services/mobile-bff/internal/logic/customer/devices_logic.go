// devices_logic.go · 我的设备（device.ListDevice party_id 过滤，S9-01 增量契约）。

package customer

import (
	"context"

	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDevicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevicesLogic {
	return &DevicesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DevicesLogic) Devices(req *types.CustomerDeviceListReq) (resp *types.CustomerDeviceListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Device.ListDevice(l.ctx, &dpb.ListDeviceReq{
		Keyword: req.Keyword,
		PartyId: partyId,
		Page:    int32(req.Page),
		Size:    int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.CustomerDeviceListResp{Total: out.Total, List: []types.CustomerDeviceItem{}}
	for _, d := range out.List {
		resp.List = append(resp.List, pbDevice(d))
	}
	return resp, nil
}
