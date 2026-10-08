package logic

import (
	"context"

	"micro-server/services/inventory/internal/model"
	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListStocktakeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStocktakeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStocktakeLogic {
	return &ListStocktakeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListStocktakeLogic) ListStocktake(in *pb.ListStocktakeReq) (*pb.ListStocktakeResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Stocktake.FindPage(l.ctx, tid, in.WarehouseId, int64(in.Status), page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListStocktakeResp{Total: total}
	for _, st := range list {
		resp.List = append(resp.List, stocktakeRecord(st, nil))
	}
	return resp, nil
}

func stocktakeRecord(st *model.Stocktake, items []*model.StocktakeItem) *pb.StocktakeRecord {
	rec := &pb.StocktakeRecord{
		StocktakeId: st.StocktakeId,
		WarehouseId: st.WarehouseId,
		Status:      int32(st.Status),
		Remark:      st.Remark.String,
		CreatedAt:   st.CreatedAt.UnixMilli(),
	}
	for _, it := range items {
		counted := int32(-1)
		if it.CountedQty.Valid {
			counted = int32(it.CountedQty.Int64)
		}
		diff := int32(0)
		if it.DiffQty.Valid {
			diff = int32(it.DiffQty.Int64)
		}
		rec.Items = append(rec.Items, &pb.StocktakeItem{
			ItemId:     it.ItemId,
			SkuId:      it.SkuId,
			BookQty:    int32(it.BookQty),
			CountedQty: counted,
			DiffQty:    diff,
		})
	}
	return rec
}
