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

type ListPricesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// SKU 价格列表(版本化,perm: catalog:price:list)
func NewListPricesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPricesLogic {
	return &ListPricesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListPricesLogic) ListPrices(req *types.PriceListReq) (resp *types.PriceListResp, err error) {
	r, err := l.svcCtx.Catalog.ListPrice(l.ctx, &catalogPb.ListPriceReq{SkuId: parseID(req.SkuID), LatestOnly: req.LatestOnly})
	if err != nil {
		return nil, err
	}
	resp = &types.PriceListResp{Total: r.Total}
	for _, p := range r.List {
		resp.List = append(resp.List, types.PriceItem{
			PriceID: strconv.FormatInt(p.PriceId, 10), SkuID: strconv.FormatInt(p.SkuId, 10),
			PriceType: p.PriceType, TierQty: int(p.TierQty), Amount: p.Amount,
			Version: int(p.Version), CreatedAt: p.CreatedAt,
		})
	}
	return resp, nil
}
