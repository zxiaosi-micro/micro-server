package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
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

// ListSlaStrategy SLA 策略列表（FR-CTR-007）。
func (l *ListSlaStrategyLogic) ListSlaStrategy(in *pb.ListSlaStrategyReq) (*pb.ListSlaStrategyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Sla.ListAll(l.ctx, tid)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListSlaStrategyResp{}
	for _, s := range list {
		resp.List = append(resp.List, buildSlaStrategyDetail(s))
	}
	return resp, nil
}
