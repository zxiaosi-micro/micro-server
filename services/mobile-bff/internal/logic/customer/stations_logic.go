// stations_logic.go · 我的场站（本人场站单回查：party 订单 → station.ListStation(order_no)）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opb "micro-server/services/order/pb"
	spb "micro-server/services/station/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type StationsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StationsLogic {
	return &StationsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *StationsLogic) Stations(req *types.NoArgsReq) (resp *types.CustomerStationListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	orders, err := l.svcCtx.Order.ListOrder(l.ctx, &opb.ListOrderReq{
		BuyerPartyId: partyId,
		Type:         "STATION",
		Page:         1,
		Size:         100,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.CustomerStationListResp{List: []types.CustomerStationItem{}}
	seen := map[string]bool{}
	for _, o := range orders.List {
		st, err := l.svcCtx.Station.ListStation(l.ctx, &spb.ListStationReq{OrderNo: o.OrderNo, Page: 1, Size: 10})
		if err != nil {
			continue
		}
		for _, s := range st.List {
			if seen[s.StationNo] {
				continue
			}
			seen[s.StationNo] = true
			resp.List = append(resp.List, pbStation(s))
		}
	}
	return resp, nil
}
