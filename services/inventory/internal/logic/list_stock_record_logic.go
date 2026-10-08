package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListStockRecordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStockRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStockRecordLogic {
	return &ListStockRecordLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListStockRecordLogic) ListStockRecord(in *pb.ListStockRecordReq) (*pb.ListStockRecordResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.StockRecord.ListPage(l.ctx, tid, in.WarehouseId, in.SkuId, in.BizType, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListStockRecordResp{Total: total}
	for _, r := range list {
		resp.List = append(resp.List, &pb.StockRecordItem{
			RecordId:        r.RecordId,
			InventoryId:     r.InventoryId,
			WarehouseId:     r.WarehouseId,
			SkuId:           r.SkuId,
			BizType:         r.BizType,
			BizNo:           r.BizNo,
			Qty:             int32(r.Qty),
			BeforeAvailable: int32(r.BeforeAvailable),
			AfterAvailable:  int32(r.AfterAvailable),
			BeforeLocked:    int32(r.BeforeLocked),
			AfterLocked:     int32(r.AfterLocked),
			Remark:          r.Remark.String,
			CreatedAt:       r.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}

var _ = model.ErrNotFound
