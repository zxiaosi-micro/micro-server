// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新建商品(perm: catalog:product:create)
func NewCreateProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProductLogic {
	return &CreateProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateProductLogic) CreateProduct(req *types.ProductCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.CreateProduct(l.ctx, &catalogPb.CreateProductReq{Name: req.Name, Category: req.Category, Remark: req.Remark})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
