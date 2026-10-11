package device

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	dpb "micro-server/services/device/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRecycleOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 回收单列表(溯源码留痕)
func NewListRecycleOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRecycleOrdersLogic {
	return &ListRecycleOrdersLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListRecycleOrdersLogic) ListRecycleOrders(req *types.RecycleListReq) (resp *types.RecycleListResp, err error) {
	out, err := l.svcCtx.Device.ListRecycleOrder(l.ctx, &dpb.ListRecycleOrderReq{
		Keyword: req.Keyword, Status: req.Status, Page: int32(req.Page), Size: int32(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.RecycleListResp{Total: out.Total, List: []types.RecycleItem{}}
	for _, r := range out.List {
		resp.List = append(resp.List, types.RecycleItem{
			RecycleId: i64s(r.RecycleId), RecycleNo: r.RecycleNo,
			AssessmentId: i64s(r.AssessmentId), DeviceId: i64s(r.DeviceId), Sn: r.Sn,
			RecyclerPartyId: r.RecyclerPartyId, ResidualValue: r.ResidualValue,
			TraceCode: r.TraceCode, EvidenceFileId: r.EvidenceFileId,
			Status: r.Status, Remark: r.Remark, CreatedAt: r.CreatedAt,
		})
	}
	return resp, nil
}
