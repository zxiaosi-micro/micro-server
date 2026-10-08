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

type CreateSlaStrategyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateSlaStrategyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSlaStrategyLogic {
	return &CreateSlaStrategyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateSlaStrategyLogic) CreateSlaStrategy(req *types.SlaCreateReq) (resp *types.SlaCreateResp, err error) {
	respOut, err := l.svcCtx.Contract.CreateSlaStrategy(l.ctx, &ctpb.CreateSlaStrategyReq{
		Code: req.Code, Name: req.Name, Level: req.Level,
		ResponseMinutes: int32(req.ResponseMinutes), ResolveMinutes: int32(req.ResolveMinutes),
	})
	if err != nil {
		return nil, err
	}
	return &types.SlaCreateResp{StrategyId: strconv.FormatInt(respOut.StrategyId, 10)}, nil
}
