package analytics

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	anpb "micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RunBIQueryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// BI 自助查询(白名单引擎)
func NewRunBIQueryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RunBIQueryLogic {
	return &RunBIQueryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RunBIQueryLogic) RunBIQuery(req *types.BIRunReq) (resp *types.BIRunResp, err error) {
	filters := make([]*anpb.BIFilter, 0, len(req.Filters))
	for _, f := range req.Filters {
		filters = append(filters, &anpb.BIFilter{Column: f.Column, Op: f.Op, Values: f.Values})
	}
	out, err := l.svcCtx.Analytics.RunBIQuery(l.ctx, &anpb.RunBIQueryReq{
		Table: req.Table, Dimensions: req.Dimensions, Metrics: req.Metrics,
		Filters: filters, OrderBy: req.OrderBy, OrderDesc: req.OrderDesc,
		Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.BIRunResp{Columns: out.Columns, Total: out.Total, Rows: []types.BIQueryRow{}}
	for _, r := range out.Rows {
		resp.Rows = append(resp.Rows, types.BIQueryRow{Values: r.Values})
	}
	return resp, nil
}
