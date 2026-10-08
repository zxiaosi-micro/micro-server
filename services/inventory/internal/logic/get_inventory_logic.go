package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetInventoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetInventoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetInventoryLogic {
	return &GetInventoryLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetInventoryLogic) GetInventory(in *pb.GetInventoryReq) (*pb.GetInventoryResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	inv, err := l.svcCtx.Models.Inventory.FindOne(l.ctx, tid, in.WarehouseId, in.SkuId)
	if err != nil {
		if err == model.ErrNotFound {
			// 未建档返回空 item（inventory_id=0）——查询语义，前端展示零库存
			return &pb.GetInventoryResp{Inventory: &pb.InventoryItem{}}, nil
		}
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.GetInventoryResp{Inventory: inventoryItem(inv)}, nil
}

func inventoryItem(inv *model.Inventory) *pb.InventoryItem {
	return &pb.InventoryItem{
		InventoryId:       inv.InventoryId,
		WarehouseId:       inv.WarehouseId,
		SkuId:             inv.SkuId,
		Available:         int32(inv.Available),
		Locked:            int32(inv.Locked),
		InTransit:         int32(inv.InTransit),
		Defective:         int32(inv.Defective),
		LowStockThreshold: int32(inv.LowStockThreshold),
		UpdatedAt:         inv.UpdatedAt.UnixMilli(),
	}
}
