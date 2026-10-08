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

type ListStockRecordsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 库存流水(perm: inventory:record:list)
func NewListStockRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStockRecordsLogic {
	return &ListStockRecordsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListStockRecordsLogic) ListStockRecords(req *types.StockRecordListReq) (resp *types.StockRecordListResp, err error) {
	r, err := l.svcCtx.Inventory.ListStockRecord(l.ctx, &inventoryPb.ListStockRecordReq{
		WarehouseId: parseID(req.WarehouseID), SkuId: parseID(req.SkuID), BizType: req.BizType,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StockRecordListResp{Total: r.Total}
	for _, rec := range r.List {
		resp.List = append(resp.List, types.StockRecordItem{
			RecordID: strconv.FormatInt(rec.RecordId, 10), InventoryID: strconv.FormatInt(rec.InventoryId, 10),
			WarehouseID: strconv.FormatInt(rec.WarehouseId, 10), SkuID: strconv.FormatInt(rec.SkuId, 10),
			BizType: rec.BizType, BizNo: rec.BizNo, Qty: int(rec.Qty),
			BeforeAvailable: int(rec.BeforeAvailable), AfterAvailable: int(rec.AfterAvailable),
			BeforeLocked: int(rec.BeforeLocked), AfterLocked: int(rec.AfterLocked),
			Remark: rec.Remark, CreatedAt: rec.CreatedAt,
		})
	}
	return resp, nil
}
