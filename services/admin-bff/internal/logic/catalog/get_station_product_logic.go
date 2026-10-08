// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStationProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 场站模板详情(含 BOM,perm: catalog:station:list)
func NewGetStationProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStationProductLogic {
	return &GetStationProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStationProductLogic) GetStationProduct(req *types.IDPath) (resp *types.StationProductDetailResp, err error) {
	r, err := l.svcCtx.Catalog.GetStationProduct(l.ctx, &catalogPb.GetStationProductReq{StationProductId: parseID(req.ID)})
	if err != nil {
		return nil, err
	}
	sp := r.StationProduct
	out := &types.StationProductDetailResp{StationProduct: types.StationProductItem{
		StationProductID: strconv.FormatInt(sp.StationProductId, 10), Name: sp.Name,
		Remark: sp.Remark, CreatedAt: sp.CreatedAt,
	}}
	for _, it := range sp.Items {
		out.StationProduct.Items = append(out.StationProduct.Items, types.BomItem{
			SkuID: strconv.FormatInt(it.SkuId, 10), SkuName: it.SkuName, Qty: int(it.Qty),
		})
	}
	return out, nil
}
