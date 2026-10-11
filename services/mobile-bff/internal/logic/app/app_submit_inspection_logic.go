// app_submit_inspection_logic.go · 巡检批量提交（定位+拍照，异常项自动转工单）。

package app

import (
	"context"
	"encoding/json"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	opsb "micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AppSubmitInspectionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAppSubmitInspectionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppSubmitInspectionLogic {
	return &AppSubmitInspectionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AppSubmitInspectionLogic) AppSubmitInspection(req *types.AppInspectionSubmitReq) (resp *types.AppInspectionSubmitResp, err error) {
	items := make([]*opsb.InspectionItemSubmit, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &opsb.InspectionItemSubmit{
			Name:      it.Name,
			Result:    it.Result,
			Anomalous: it.Anomalous,
		})
	}
	photos, _ := json.Marshal(req.Photos)
	out, err := l.svcCtx.Ops.SubmitInspectionTask(l.ctx, &opsb.SubmitInspectionTaskReq{
		TaskId:   parseI64(req.ID),
		Items:    items,
		Location: req.Location,
		Photos:   string(photos),
		Remark:   req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.AppInspectionSubmitResp{
		AnomalyCount: int64(out.AnomalyCount),
		TicketIds:    jsonStrings(out.TicketIds),
	}, nil
}
