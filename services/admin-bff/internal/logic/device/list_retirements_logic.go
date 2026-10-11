package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRetirementsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 退役评估列表
func NewListRetirementsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRetirementsLogic {
	return &ListRetirementsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListRetirementsLogic) ListRetirements(req *types.RetirementListReq) (resp *types.RetirementListResp, err error) {
	out, err := l.svcCtx.Device.ListRetirementAssessment(l.ctx, &dpb.ListRetirementAssessmentReq{
		Keyword: req.Keyword, Grade: req.Grade, Disposition: req.Disposition,
		Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.RetirementListResp{Total: out.Total, List: []types.RetirementItem{}}
	for _, r := range out.List {
		resp.List = append(resp.List, types.RetirementItem{
			AssessmentId: i64s(r.AssessmentId), AssessmentNo: r.AssessmentNo,
			DeviceId: i64s(r.DeviceId), Sn: r.Sn, Round: int(r.Round),
			Soh: r.Soh, CycleCount: int(r.CycleCount), ResistanceMohm: r.ResistanceMohm,
			ConsistencyOk: r.ConsistencyOk, SafetyJson: r.SafetyJson,
			Grade: r.Grade, Disposition: r.Disposition, InspectionTicketNo: r.InspectionTicketNo,
			Remark: r.Remark, CreatedAt: r.CreatedAt,
		})
	}
	return resp, nil
}
