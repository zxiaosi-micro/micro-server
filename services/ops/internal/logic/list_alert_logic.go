package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAlertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAlertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertLogic {
	return &ListAlertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 告警（FR-OPS-002/003/009/010）----
func (l *ListAlertLogic) ListAlert(in *pb.ListAlertReq) (*pb.ListAlertResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Alert.ListPage(l.ctx, tid, in.Status, in.Level, in.Source, in.StationId, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListAlertResp{Total: total}
	for _, a := range list {
		resp.List = append(resp.List, pbAlert(a))
	}
	return resp, nil
}
