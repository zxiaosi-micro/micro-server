package logic

import (
	"context"
	"strconv"

	"micro-server/services/analytics/internal/svc"
	"micro-server/services/analytics/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type RequestExportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestExportLogic {
	return &RequestExportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RequestExport 导出申请：任务落库 + export_requested 事件（同事务 Outbox）。
func (l *RequestExportLogic) RequestExport(in *pb.RequestExportReq) (*pb.RequestExportResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	taskID, err := RequestExportCore(l.ctx, l.svcCtx, tid, in.ReportType, in.Format, in.DateStart, in.DateEnd, ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	return &pb.RequestExportResp{ExportId: strconv.FormatInt(taskID, 10)}, nil
}
