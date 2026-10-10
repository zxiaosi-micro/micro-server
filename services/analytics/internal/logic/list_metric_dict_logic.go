package logic

import (
	"context"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListMetricDictLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMetricDictLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMetricDictLogic {
	return &ListMetricDictLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListMetricDict 指标字典（口径正本，S8-02）。
func (l *ListMetricDictLogic) ListMetricDict(in *pb.ListMetricDictReq) (*pb.ListMetricDictResp, error) {
	rows, err := l.svcCtx.Models.Dict.List(l.ctx, in.ReportType)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListMetricDictResp{Total: int64(len(rows))}
	for _, r := range rows {
		resp.List = append(resp.List, &pb.MetricDictItem{
			MetricCode: r.MetricCode, Name: r.Name, ReportType: r.ReportType,
			Unit: r.Unit, Caliber: r.Caliber, SourceEvent: r.SourceEvent,
			Recon: r.Recon == 1, Sort: r.Sort,
		})
	}
	return resp, nil
}
