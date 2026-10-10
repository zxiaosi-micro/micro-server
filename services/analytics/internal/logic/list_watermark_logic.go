package logic

import (
	"context"
	"strconv"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWatermarkLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWatermarkLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWatermarkLogic {
	return &ListWatermarkLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListWatermark 消费水位（运维/冒烟观测）。
func (l *ListWatermarkLogic) ListWatermark(in *pb.ListWatermarkReq) (*pb.ListWatermarkResp, error) {
	rows, err := l.svcCtx.Models.Watermark.List(l.ctx)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListWatermarkResp{}
	for _, r := range rows {
		item := &pb.WatermarkItem{
			Topic: r.Topic, ConsumerGroup: r.ConsumerGroup, LastEventId: r.LastEventID,
			ProcessedCount: r.ProcessedCount, UpdatedAt: strconv.FormatInt(r.UpdatedAt.UnixMilli(), 10),
		}
		if r.LastStatDate != nil {
			item.LastStatDate = r.LastStatDate.Format("2006-01-02")
		}
		resp.List = append(resp.List, item)
	}
	if v, err := l.svcCtx.Models.Meta.GetVersion(l.ctx); err == nil {
		resp.RebuildVersion = v
	}
	return resp, nil
}
