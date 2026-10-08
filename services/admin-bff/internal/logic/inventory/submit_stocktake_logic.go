// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubmitStocktakeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 提交实盘(perm: inventory:stocktake:submit)
func NewSubmitStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitStocktakeLogic {
	return &SubmitStocktakeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubmitStocktakeLogic) SubmitStocktake(req *types.StocktakeSubmitReq) (resp *types.SimpleResp, err error) {
	items := make([]*inventoryPb.SubmitStocktakeCountItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &inventoryPb.SubmitStocktakeCountItem{SkuId: parseID(it.SkuID), CountedQty: int32(it.CountedQty)})
	}
	_, err = l.svcCtx.Inventory.SubmitStocktakeCount(l.ctx, &inventoryPb.SubmitStocktakeCountReq{
		StocktakeId: parseID(req.Id), Items: items,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
