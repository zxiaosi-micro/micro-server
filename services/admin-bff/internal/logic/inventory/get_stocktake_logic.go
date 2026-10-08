// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStocktakeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 盘点单详情(perm: inventory:stocktake:list)
func NewGetStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStocktakeLogic {
	return &GetStocktakeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStocktakeLogic) GetStocktake(req *types.IDPath) (resp *types.StocktakeDetailResp, err error) {
	r, err := l.svcCtx.Inventory.GetStocktake(l.ctx, &inventoryPb.GetStocktakeReq{StocktakeId: parseID(req.ID)})
	if err != nil {
		return nil, err
	}
	s := r.Stocktake
	out := &types.StocktakeDetailResp{Stocktake: types.StocktakeRecord{
		StocktakeID: strconv.FormatInt(s.StocktakeId, 10), WarehouseID: strconv.FormatInt(s.WarehouseId, 10),
		Status: int(s.Status), Remark: s.Remark, CreatedAt: s.CreatedAt,
	}}
	for _, it := range s.Items {
		out.Stocktake.Items = append(out.Stocktake.Items, types.StocktakeItem{
			ItemID: strconv.FormatInt(it.ItemId, 10), SkuID: strconv.FormatInt(it.SkuId, 10),
			BookQty: int(it.BookQty), CountedQty: int(it.CountedQty), DiffQty: int(it.DiffQty),
		})
	}
	return out, nil
}
