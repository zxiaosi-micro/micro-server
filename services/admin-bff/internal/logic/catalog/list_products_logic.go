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

type ListProductsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商品列表(perm: catalog:product:list)
func NewListProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProductsLogic {
	return &ListProductsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListProductsLogic) ListProducts(req *types.ProductListReq) (resp *types.ProductListResp, err error) {
	r, err := l.svcCtx.Catalog.ListProduct(l.ctx, &catalogPb.ListProductReq{
		Keyword: req.Keyword, Status: int32(req.Status), Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.ProductListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.ProductItem{
			ProductID: strconv.FormatInt(p.ProductId, 10), Name: p.Name, Category: p.Category,
			Status: int(p.Status), Remark: p.Remark, CreatedAt: p.CreatedAt,
		})
	}
	return resp, nil
}
