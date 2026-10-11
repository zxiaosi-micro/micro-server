package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRetirementAssessmentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 发起退役评估(复用 ops 检测工单)
func NewCreateRetirementAssessmentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRetirementAssessmentLogic {
	return &CreateRetirementAssessmentLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateRetirementAssessmentLogic) CreateRetirementAssessment(req *types.RetirementCreateReq) (resp *types.RetirementCreateResp, err error) {
	out, err := l.svcCtx.Device.CreateRetirementAssessment(l.ctx, &dpb.CreateRetirementAssessmentReq{
		DeviceId: parseI64(req.Id), InspectionTicketNo: req.InspectionTicketNo, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.RetirementCreateResp{
		AssessmentId: i64s(out.AssessmentId), AssessmentNo: out.AssessmentNo, Round: int(out.Round),
	}, nil
}
