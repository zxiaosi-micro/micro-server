// device_detail_logic.go · 设备详情（影子+质保+场站归属，FR-DEV-006）。

package customer

import (
	"context"

	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	spb "micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeviceDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeviceDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeviceDetailLogic {
	return &DeviceDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeviceDetailLogic) DeviceDetail(req *types.IDPath) (resp *types.CustomerDeviceDetail, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	dev, err := ownDeviceByID(l.ctx, l.svcCtx, parseI64(req.ID), partyId)
	if err != nil {
		return nil, err
	}

	resp = &types.CustomerDeviceDetail{
		Device:     pbDevice(dev),
		Shadow:     types.ShadowView{Metrics: []types.MetricView{}},
		Warranties: []types.WarrantyView{},
	}

	// 影子（弱网可查最近状态）
	if sh, err := l.svcCtx.Device.GetDeviceShadow(l.ctx, &dpb.GetDeviceShadowReq{Sn: dev.Sn}); err == nil {
		resp.Shadow = pbShadow(sh)
	}

	// 质保（设备层）
	if w, err := l.svcCtx.Contract.GetWarrantyByTarget(l.ctx, &ctpb.GetWarrantyByTargetReq{
		TargetType: "DEVICE", TargetId: dev.DeviceId,
	}); err == nil {
		for _, wi := range w.List {
			resp.Warranties = append(resp.Warranties, pbWarranty(wi))
		}
	}

	// 场站归属
	if st, err := l.svcCtx.Station.GetDeviceStation(l.ctx, &spb.GetDeviceStationReq{DeviceId: dev.DeviceId}); err == nil && st.GetStation() != nil {
		resp.Station = types.StationBrief{
			StationId: i64s(st.Station.StationId),
			StationNo: st.Station.StationNo,
			Name:      st.Station.StationName,
		}
	}
	return resp, nil
}
