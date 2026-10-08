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

type ListSkusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// SKU 列表(perm: catalog:sku:list)
func NewListSkusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSkusLogic {
	return &ListSkusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListSkusLogic) ListSkus(req *types.SkuListReq) (resp *types.SkuListResp, err error) {
	r, err := l.svcCtx.Catalog.ListSKU(l.ctx, &catalogPb.ListSKUReq{
		ProductId: parseID(req.ProductID), Keyword: req.Keyword, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.SkuListResp{Total: r.Total}
	for _, s := range r.List {
		resp.List = append(resp.List, types.SkuItem{
			SkuID: strconv.FormatInt(s.SkuId, 10), ProductID: strconv.FormatInt(s.ProductId, 10),
			Code: s.Code, Name: s.Name, Type: s.Type, Spec: s.Spec,
			Status: int(s.Status), CreatedAt: s.CreatedAt,
		})
	}
	return resp, nil
}
