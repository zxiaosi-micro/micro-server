// station_detail_logic.go · 场站详情（归属校验 + 设备影子聚合监控，FR-STN-005）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opb "micro-server/services/order/pb"
	spb "micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type StationDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStationDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StationDetailLogic {
	return &StationDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *StationDetailLogic) StationDetail(req *types.IDPath) (resp *types.CustomerStationDetail, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	stationId := parseI64(req.ID)

	// 归属校验：场站来源单须属于本人（station.party 无直连，经 order_no 回查订单买方）
	st, err := l.svcCtx.Station.GetStation(l.ctx, &spb.GetStationReq{StationId: stationId})
	if err != nil {
		return nil, err
	}
	if o, err := l.svcCtx.Order.GetOrder(l.ctx, &opb.GetOrderReq{OrderNo: st.GetStation().GetOrderNo()}); err != nil ||
		o.GetOrder().GetBuyerPartyId() != partyId {
		return nil, errcode.ErrPermissionDenied
	}

	resp = &types.CustomerStationDetail{
		Station: pbStation(st.Station),
		Devices: []types.CustomerStationDevice{},
	}
	if mon, err := l.svcCtx.Station.GetStationMonitor(l.ctx, &spb.GetStationMonitorReq{StationId: stationId}); err == nil {
		for _, it := range mon.Items {
			resp.Devices = append(resp.Devices, types.CustomerStationDevice{
				DeviceId:    i64s(it.DeviceId),
				Sn:          it.Sn,
				Online:      it.Online,
				Soc:         it.Soc,
				Power:       it.Power,
				Voltage:     it.Voltage,
				Temperature: it.Temperature,
			})
		}
	}
	return resp, nil
}
