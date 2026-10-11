// extension_skus_logic.go · 延保商品（EXT_WARRANTY SKU + 零售价；价格缺省回退展示空）。

package customer

import (
	"context"

	cpb "micro-server/services/catalog/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ExtensionSkusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExtensionSkusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExtensionSkusLogic {
	return &ExtensionSkusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ExtensionSkusLogic) ExtensionSkus(req *types.NoArgsReq) (resp *types.ExtensionSkuListResp, err error) {
	out, err := l.svcCtx.Catalog.ListSKU(l.ctx, &cpb.ListSKUReq{Page: 1, Size: 100})
	if err != nil {
		return nil, err
	}
	resp = &types.ExtensionSkuListResp{List: []types.ExtensionSkuView{}}
	for _, s := range out.List {
		if s.Type != "EXT_WARRANTY" {
			continue
		}
		view := types.ExtensionSkuView{SkuId: i64s(s.SkuId), Code: s.Code, Name: s.Name, Spec: s.Spec}
		if p, err := l.svcCtx.Catalog.ListPrice(l.ctx, &cpb.ListPriceReq{SkuId: s.SkuId, LatestOnly: true, Page: 1, Size: 10}); err == nil {
			for _, pr := range p.List {
				if pr.PriceType == "RETAIL" {
					view.Price = pr.Amount
					break
				}
			}
		}
		resp.List = append(resp.List, view)
	}
	return resp, nil
}
