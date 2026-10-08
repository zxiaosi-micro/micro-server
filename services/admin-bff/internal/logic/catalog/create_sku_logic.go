// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateSkuLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建 SKU(perm: catalog:sku:create)
func NewCreateSkuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSkuLogic {
	return &CreateSkuLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateSkuLogic) CreateSku(req *types.SkuCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.CreateSKU(l.ctx, &catalogPb.CreateSKUReq{
		ProductId: parseID(req.ProductID), Code: req.Code, Name: req.Name, Type: req.Type, Spec: req.Spec,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
