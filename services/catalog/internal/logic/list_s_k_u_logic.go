package logic

import (
	"context"

	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListSKULogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListSKULogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListSKULogic {
	return &ListSKULogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListSKULogic) ListSKU(in *pb.ListSKUReq) (*pb.ListSKUResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Sku.FindPage(l.ctx, tid, in.ProductId, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListSKUResp{Total: total}
	for _, s := range list {
		resp.List = append(resp.List, &pb.SkuItem{
			SkuId:     s.SkuId,
			ProductId: s.ProductId,
			Code:      s.Code,
			Name:      s.Name,
			Type:      s.Type,
			Spec:      s.Spec.String,
			Status:    int32(s.Status),
			CreatedAt: s.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
