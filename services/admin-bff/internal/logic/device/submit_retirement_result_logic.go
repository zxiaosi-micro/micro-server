package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitRetirementResultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 退役检测结果录入(录入即评级 A/B/C)
func NewSubmitRetirementResultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitRetirementResultLogic {
	return &SubmitRetirementResultLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SubmitRetirementResultLogic) SubmitRetirementResult(req *types.RetirementSubmitReq) (resp *types.RetirementSubmitResp, err error) {
	items := make([]*dpb.SafetyItem, 0, len(req.SafetyItems))
	for _, it := range req.SafetyItems {
		items = append(items, &dpb.SafetyItem{Item: it.Item, Pass: it.Pass})
	}
	out, err := l.svcCtx.Device.SubmitRetirementResult(l.ctx, &dpb.SubmitRetirementResultReq{
		AssessmentId: parseI64(req.Id), Soh: req.Soh, CycleCount: int32(req.CycleCount),
		ResistanceMohm: req.ResistanceMohm, ConsistencyOk: req.ConsistencyOk,
		SafetyItems: items, MeasuredAt: req.MeasuredAt, Source: req.Source,
	})
	if err != nil {
		return nil, err
	}
	return &types.RetirementSubmitResp{Grade: out.Grade, Reason: out.Reason}, nil
}
