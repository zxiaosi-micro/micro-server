// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateStationProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建场站模板(perm: catalog:station:create)
func NewCreateStationProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateStationProductLogic {
	return &CreateStationProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateStationProductLogic) CreateStationProduct(req *types.StationProductCreateReq) (resp *types.SimpleResp, err error) {
	items := make([]*catalogPb.BomItem, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &catalogPb.BomItem{SkuId: parseID(it.SkuID), Qty: int32(it.Qty)})
	}
	_, err = l.svcCtx.Catalog.CreateStationProduct(l.ctx, &catalogPb.CreateStationProductReq{
		Name: req.Name, Remark: req.Remark, Items: items,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
