// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package contract

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListSlaStrategiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListSlaStrategiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSlaStrategiesLogic {
	return &ListSlaStrategiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSlaStrategiesLogic) ListSlaStrategies() (resp *types.SlaListResp, err error) {
	respOut, err := l.svcCtx.Contract.ListSlaStrategy(l.ctx, &ctpb.ListSlaStrategyReq{})
	if err != nil {
		return nil, err
	}
	out := &types.SlaListResp{}
	for _, s := range respOut.List {
		out.List = append(out.List, types.SlaStrategyView{
			StrategyId: strconv.FormatInt(s.StrategyId, 10), Code: s.Code, Name: s.Name,
			Level: s.Level, ResponseMinutes: int(s.ResponseMinutes),
			ResolveMinutes: int(s.ResolveMinutes), CreatedAt: s.CreatedAt,
		})
	}
	return out, nil
}
