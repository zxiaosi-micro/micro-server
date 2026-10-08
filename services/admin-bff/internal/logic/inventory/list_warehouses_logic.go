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

type ListWarehousesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 仓库列表(perm: inventory:warehouse:list)
func NewListWarehousesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWarehousesLogic {
	return &ListWarehousesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListWarehousesLogic) ListWarehouses(req *types.WarehouseListReq) (resp *types.WarehouseListResp, err error) {
	r, err := l.svcCtx.Inventory.ListWarehouse(l.ctx, &inventoryPb.ListWarehouseReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.WarehouseListResp{Total: r.Total}
	for _, w := range r.List {
		resp.List = append(resp.List, types.WarehouseItem{
			WarehouseID: strconv.FormatInt(w.WarehouseId, 10), Code: w.Code, Name: w.Name,
			Address: w.Address, Status: int(w.Status), CreatedAt: w.CreatedAt,
		})
	}
	return resp, nil
}
