// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetPriceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 设置价格(新版本落库,perm: catalog:price:set)
func NewSetPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetPriceLogic {
	return &SetPriceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SetPriceLogic) SetPrice(req *types.PriceSetReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.SetPrice(l.ctx, &catalogPb.SetPriceReq{
		SkuId: parseID(req.Id), PriceType: req.PriceType, TierQty: int32(req.TierQty), Amount: req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
