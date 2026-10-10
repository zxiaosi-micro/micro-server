package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	contractpb "micro-server/services/contract/pb"
)

type ListSlaStrategyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSlaStrategyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSlaStrategyLogic {
	return &ListSlaStrategyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- SLA（FR-OPS-006；策略正本 contract，本服务计时）----
func (l *ListSlaStrategyLogic) ListSlaStrategy(in *pb.ListSlaStrategyReq) (*pb.ListSlaStrategyResp, error) {
	if l.svcCtx.Contract == nil {
		return &pb.ListSlaStrategyResp{}, nil
	}
	out, err := l.svcCtx.Contract.ListSlaStrategy(l.ctx, &contractpb.ListSlaStrategyReq{})
	if err != nil {
		return nil, err
	}
	resp := &pb.ListSlaStrategyResp{}
	for _, s := range out.List {
		resp.List = append(resp.List, &pb.SlaStrategyView{
			StrategyId: s.StrategyId, Code: s.Code, Name: s.Name, Level: s.Level,
			ResponseMinutes: s.ResponseMinutes, ResolveMinutes: s.ResolveMinutes,
		})
	}
	return resp, nil
}
