package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateRecycleOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 回收单补建(C 级评估漏建补录)
func NewCreateRecycleOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateRecycleOrderLogic {
	return &CreateRecycleOrderLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateRecycleOrderLogic) CreateRecycleOrder(req *types.RecycleCreateReq) (resp *types.RecycleCreateResp, err error) {
	out, err := l.svcCtx.Device.CreateRecycleOrder(l.ctx, &dpb.CreateRecycleOrderReq{
		AssessmentId: parseI64(req.Id), RecyclerPartyId: req.RecyclerPartyId,
		ResidualValue: req.ResidualValue, EvidenceFileId: req.EvidenceFileId, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.RecycleCreateResp{
		RecycleId: i64s(out.RecycleId), RecycleNo: out.RecycleNo, TraceCode: out.TraceCode,
	}, nil
}
