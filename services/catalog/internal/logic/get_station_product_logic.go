package logic

import (
	"context"

	"micro-server/services/catalog/internal/model"
	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetStationProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStationProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationProductLogic {
	return &GetStationProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetStationProduct 取场站模板（含 BOM 明细，SKU 名称服务内聚合展示）。
func (l *GetStationProductLogic) GetStationProduct(in *pb.GetStationProductReq) (*pb.GetStationProductResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	sp, err := l.svcCtx.Models.StationProduct.FindOne(l.ctx, tid, in.StationProductId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errStationNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	items, err := l.svcCtx.Models.StationProductMdl.ListByStation(l.ctx, tid, sp.StationProductId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	respItems := make([]*pb.BomItem, 0, len(items))
	for _, it := range items {
		name := ""
		if sku, serr := l.svcCtx.Models.Sku.FindOne(l.ctx, tid, it.SkuId); serr == nil {
			name = sku.Name
		}
		respItems = append(respItems, &pb.BomItem{SkuId: it.SkuId, SkuName: name, Qty: int32(it.Qty)})
	}
	return &pb.GetStationProductResp{StationProduct: &pb.StationProductItem{
		StationProductId: sp.StationProductId,
		Name:             sp.Name,
		Remark:           sp.Remark.String,
		Items:            respItems,
		CreatedAt:        sp.CreatedAt.UnixMilli(),
	}}, nil
}
