// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStocktakeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建盘点单(快照账面,perm: inventory:stocktake:create)
func NewCreateStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStocktakeLogic {
	return &CreateStocktakeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateStocktakeLogic) CreateStocktake(req *types.StocktakeCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.CreateStocktake(l.ctx, &inventoryPb.CreateStocktakeReq{
		WarehouseId: parseID(req.WarehouseID), Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
