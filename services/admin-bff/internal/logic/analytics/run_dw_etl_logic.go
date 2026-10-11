package analytics

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	anpb "micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunDwEtlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 数仓 ETL 手动触发/补历史(重跑幂等)
func NewRunDwEtlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunDwEtlLogic {
	return &RunDwEtlLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RunDwEtlLogic) RunDwEtl(req *types.DwEtlRunReq) (resp *types.DwEtlRunResp, err error) {
	out, err := l.svcCtx.Analytics.RunDwEtl(l.ctx, &anpb.RunDwEtlReq{
		DateStart: req.DateStart, DateEnd: req.DateEnd,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.DwEtlRunResp{Days: []types.DwEtlDay{}}
	for _, d := range out.Days {
		resp.Days = append(resp.Days, types.DwEtlDay{
			EtlDate: d.EtlDate, DimDateRows: d.DimDateRows, DimTenantRows: d.DimTenantRows,
			DimPartyRows: d.DimPartyRows, FactOrderRows: d.FactOrderRows,
			FactPaymentRows: d.FactPaymentRows, CostMs: d.CostMs,
		})
	}
	return resp, nil
}
