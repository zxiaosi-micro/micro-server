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

type ListStationProductsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 场站模板列表(perm: catalog:station:list)
func NewListStationProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationProductsLogic {
	return &ListStationProductsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListStationProductsLogic) ListStationProducts(req *types.StationProductListReq) (resp *types.StationProductListResp, err error) {
	r, err := l.svcCtx.Catalog.ListStationProduct(l.ctx, &catalogPb.ListStationProductReq{
		Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.StationProductListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.StationProductItem{
			StationProductID: strconv.FormatInt(s.StationProductId, 10), Name: s.Name,
			Remark: s.Remark, CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}
