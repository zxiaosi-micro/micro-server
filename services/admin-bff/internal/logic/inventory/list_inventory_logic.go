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

type ListInventoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 库存列表(四态,perm: inventory:inventory:list)
func NewListInventoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInventoryLogic {
	return &ListInventoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListInventoryLogic) ListInventory(req *types.InventoryListReq) (resp *types.InventoryListResp, err error) {
	r, err := l.svcCtx.Inventory.ListInventory(l.ctx, &inventoryPb.ListInventoryReq{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), LowStockOnly: req.LowOnly,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InventoryListResp{Total: r.Total}
	for _, i := range r.List {
		resp.List = append(resp.List, types.InventoryItem{
			InventoryID: strconv.FormatInt(i.InventoryId, 10), WarehouseID: strconv.FormatInt(i.WarehouseId, 10),
			SkuID: strconv.FormatInt(i.SkuId, 10), Available: int(i.Available), Locked: int(i.Locked),
			InTransit: int(i.InTransit), Defective: int(i.Defective),
			LowStockThreshold: int(i.LowStockThreshold), UpdatedAt: i.UpdatedAt,
		})
	}
	return resp, nil
}
