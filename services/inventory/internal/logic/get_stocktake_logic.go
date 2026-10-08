package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetStocktakeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStocktakeLogic {
	return &GetStocktakeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetStocktakeLogic) GetStocktake(in *pb.GetStocktakeReq) (*pb.GetStocktakeResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	st, err := l.svcCtx.Models.Stocktake.FindOne(l.ctx, tid, in.StocktakeId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStocktakeNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	items, err := l.svcCtx.Models.StocktakeItem.ListByStocktake(l.ctx, tid, in.StocktakeId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.GetStocktakeResp{Stocktake: stocktakeRecord(st, items)}, nil
}
