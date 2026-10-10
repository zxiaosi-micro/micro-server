package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PendingAlertsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPendingAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PendingAlertsLogic {
	return &PendingAlertsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// PendingAlerts 轮询增量接口（轻量 <5KB / P95<100ms，FR-OPS-010）。
func (l *PendingAlertsLogic) PendingAlerts(in *pb.PendingAlertsReq) (*pb.PendingAlertsResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	limit := int(in.Limit)
	if limit <= 0 || limit > 50 {
		limit = 20 // 单响应 <5KB 口径
	}
	items, err := l.svcCtx.Models.Alert.ListPending(l.ctx, tid, in.StationId, in.SinceId, limit)
	if err != nil {
		return nil, err
	}
	resp := &pb.PendingAlertsResp{
		UnreadCount: getAlertUnread(l.svcCtx, tid, in.StationId),
	}
	for _, a := range items {
		resp.MaxId = a.AlertId
		resp.Items = append(resp.Items, &pb.AlertBrief{
			AlertId:        a.AlertId,
			Level:          a.Level,
			Source:         a.Source,
			Metric:         a.Metric,
			Title:          a.Title,
			StationId:      a.StationId,
			StationName:    a.StationName,
			AggregateCount: int32(a.AggregateCount),
			Status:         a.Status,
			FirstSeenAt:    a.FirstSeenAt.UnixMilli(),
		})
	}
	return resp, nil
}
