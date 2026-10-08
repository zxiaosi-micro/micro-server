// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApproveStocktakeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 盘点审批(通过生成账面调整流水,perm: inventory:stocktake:approve)
func NewApproveStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveStocktakeLogic {
	return &ApproveStocktakeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApproveStocktakeLogic) ApproveStocktake(req *types.StocktakeApproveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Inventory.ApproveStocktake(l.ctx, &inventoryPb.ApproveStocktakeReq{
		StocktakeId: parseID(req.Id), Approve: req.Approve, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
