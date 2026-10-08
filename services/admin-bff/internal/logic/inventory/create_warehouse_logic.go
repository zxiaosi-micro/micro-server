// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateWarehouseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建仓库(perm: inventory:warehouse:create)
func NewCreateWarehouseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateWarehouseLogic {
	return &CreateWarehouseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateWarehouseLogic) CreateWarehouse(req *types.WarehouseCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.CreateWarehouse(l.ctx, &inventoryPb.CreateWarehouseReq{Code: req.Code, Name: req.Name, Address: req.Address})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
