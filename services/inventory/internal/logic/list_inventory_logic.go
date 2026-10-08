package logic

import (
	"context"

	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListInventoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListInventoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListInventoryLogic {
	return &ListInventoryLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListInventoryLogic) ListInventory(in *pb.ListInventoryReq) (*pb.ListInventoryResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Inventory.ListPage(l.ctx, tid, in.WarehouseId, in.SkuId, in.LowStockOnly, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListInventoryResp{Total: total}
	for _, inv := range list {
		resp.List = append(resp.List, inventoryItem(inv))
	}
	return resp, nil
}
