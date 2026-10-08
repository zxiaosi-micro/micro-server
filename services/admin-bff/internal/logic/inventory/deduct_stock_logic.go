// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package inventory

import (
	"context"
	"strconv"
	"time"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	inventoryPb "micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type DeductStockLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 出库发货(perm: inventory:stock:deduct)
func NewDeductStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeductStockLogic {
	return &DeductStockLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeductStockLogic) DeductStock(req *types.StockOpReq) (resp *types.StockOpResp, err error) {
	bizNo := req.BizNo
	if bizNo == "" {
		bizNo = "ADMIN" + strconv.FormatInt(ctxkit.UID(l.ctx), 10) + time.Now().Format("150405.000000000")
	}
	r, err := l.svcCtx.Inventory.DeductLocked(l.ctx, &inventoryPb.DeductLockedReq{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), Qty: int32(req.Qty),
		BizType: opBizType(req.BizType), BizNo: bizNo, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.StockOpResp{RecordID: strconv.FormatInt(r.RecordId, 10)}, nil
}
