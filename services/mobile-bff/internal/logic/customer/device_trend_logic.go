// device_trend_logic.go · 遥测趋势（TDengine 热点五指标 1min 桶降采样；只读直查，业务库零直连）。

package customer

import (
	"context"
	"strconv"
	"time"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeviceTrendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeviceTrendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeviceTrendLogic {
	return &DeviceTrendLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeviceTrendLogic) DeviceTrend(req *types.TrendReq) (resp *types.TrendResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	dev, err := ownDeviceByID(l.ctx, l.svcCtx, parseI64(req.ID), partyId)
	if err != nil {
		return nil, err
	}
	rows, err := l.svcCtx.Td.Trend(l.ctx, dev.ProductKey, dev.Sn, req.Hours)
	if err != nil {
		return nil, err
	}
	resp = &types.TrendResp{Points: []types.TrendPoint{}}
	for _, row := range rows {
		if len(row) < 6 {
			continue
		}
		ts, _ := row[0].(time.Time)
		resp.Points = append(resp.Points, types.TrendPoint{
			Ts:          ts.Format("2006-01-02 15:04:05"),
			Soc:         toF64(row[1]),
			Voltage:     toF64(row[2]),
			Current:     toF64(row[3]),
			Temperature: toF64(row[4]),
			Power:       toF64(row[5]),
		})
	}
	return resp, nil
}

// toF64 TDengine REST 数值列（JSON number / string 兼容）。
func toF64(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case nil:
		return 0
	default:
		return 0
	}
}
