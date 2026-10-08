package logic

import (
	"context"

	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListPriceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListPriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListPriceLogic {
	return &ListPriceLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ListPrice SKU 价格列表：latest_only=true 价目表视图（各键最新版本）；false 全版本历史（版本切换）。
func (l *ListPriceLogic) ListPrice(in *pb.ListPriceReq) (*pb.ListPriceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.SkuId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("sku_id 必填")
	}
	list, err := l.svcCtx.Models.Price.ListBySku(l.ctx, tid, in.SkuId, in.LatestOnly)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListPriceResp{}
	for _, p := range list {
		resp.List = append(resp.List, &pb.PriceItem{
			PriceId:   p.PriceId,
			SkuId:     p.SkuId,
			PriceType: p.PriceType,
			TierQty:   int32(p.TierQty),
			Amount:    formatAmount(p.Amount),
			Version:   int32(p.Version),
			CreatedAt: p.CreatedAt.UnixMilli(),
		})
	}
	resp.Total = int64(len(resp.List))
	return resp, nil
}
