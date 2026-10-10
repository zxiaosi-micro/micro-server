package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type SubmitInspectionTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitInspectionTaskLogic {
	return &SubmitInspectionTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SubmitInspectionTask 批量提交明细；异常项（anomalous=true）自动生成工单（type=INSPECTION）。
func (l *SubmitInspectionTaskLogic) SubmitInspectionTask(in *pb.SubmitInspectionTaskReq) (*pb.SubmitInspectionTaskResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	uid := ctxkit.UID(l.ctx)
	items := make([]InspectionItemIn, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, InspectionItemIn{Name: it.Name, Result: it.Result, Anomalous: it.Anomalous})
	}
	anomalyCount, ticketIds, err := SubmitInspectionCore(l.ctx, l.svcCtx, tid, in.TaskId, uid, in.Location, in.Photos, in.Remark, items)
	if err != nil {
		return nil, err
	}
	return &pb.SubmitInspectionTaskResp{AnomalyCount: int32(anomalyCount), TicketIds: ticketIds}, nil
}
