// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package catalog

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	catalogPb "micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpsertWarrantyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 质保策略 Upsert(perm: catalog:warranty:update)
func NewUpsertWarrantyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertWarrantyLogic {
	return &UpsertWarrantyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpsertWarrantyLogic) UpsertWarranty(req *types.WarrantyPolicyUpsertReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Catalog.UpsertWarrantyPolicy(l.ctx, &catalogPb.UpsertWarrantyPolicyReq{
		SkuId: parseID(req.Id), PeriodMonths: int32(req.PeriodMonths), StartRule: req.StartRule,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
