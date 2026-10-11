package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DecideRetirementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 退役评级处置(A→二次销售/B→降容备电/C→报废回收)
func NewDecideRetirementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DecideRetirementLogic {
	return &DecideRetirementLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DecideRetirementLogic) DecideRetirement(req *types.RetirementDecideReq) (resp *types.RetirementDecideResp, err error) {
	out, err := l.svcCtx.Device.DecideRetirement(l.ctx, &dpb.DecideRetirementReq{
		AssessmentId: parseI64(req.Id), Target: req.Target,
		RecyclerPartyId: req.RecyclerPartyId, ResidualValue: req.ResidualValue, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.RetirementDecideResp{
		Grade: out.Grade, Disposition: out.Disposition,
		RecycleNo: out.RecycleNo, TraceCode: out.TraceCode, DeviceStatus: out.DeviceStatus,
	}, nil
}
