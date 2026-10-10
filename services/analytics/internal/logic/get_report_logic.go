package logic

import (
	"context"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetReportLogic {
	return &GetReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetReport 报表查询（读模型服务面；租户 scoped）。
func (l *GetReportLogic) GetReport(in *pb.GetReportReq) (*pb.GetReportResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	rows, err := GetReportRows(l.ctx, l.svcCtx, tid, in.ReportType, in.DateStart, in.DateEnd,
		in.StationId, in.WarehouseId, in.Channel)
	if err != nil {
		return nil, err
	}
	resp := &pb.GetReportResp{Total: int64(len(rows))}
	for _, r := range rows {
		resp.Rows = append(resp.Rows, &pb.ReportRow{
			StatDate: r.StatDate, StationId: r.StationID, WarehouseId: r.WarehouseID,
			Channel: r.Channel, Metrics: r.Metrics,
		})
	}
	return resp, nil
}
