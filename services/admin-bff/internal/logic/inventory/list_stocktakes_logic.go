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

type ListStocktakesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 盘点单列表(perm: inventory:stocktake:list)
func NewListStocktakesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStocktakesLogic {
	return &ListStocktakesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListStocktakesLogic) ListStocktakes(req *types.StocktakeListReq) (resp *types.StocktakeListResp, err error) {
	r, err := l.svcCtx.Inventory.ListStocktake(l.ctx, &inventoryPb.ListStocktakeReq{
		WarehouseId: parseID(req.WarehouseID), Status: int32(req.Status), Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StocktakeListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StocktakeRecord{
			StocktakeID: strconv.FormatInt(s.StocktakeId, 10), WarehouseID: strconv.FormatInt(s.WarehouseId, 10),
			Status: int(s.Status), Remark: s.Remark, CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}
