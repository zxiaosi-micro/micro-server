package logic

import (
	"context"

	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProductLogic {
	return &ListProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListProductLogic) ListProduct(in *pb.ListProductReq) (*pb.ListProductResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Product.FindPage(l.ctx, tid, in.Keyword, int64(in.Status), page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListProductResp{Total: total}
	for _, p := range list {
		resp.List = append(resp.List, &pb.ProductItem{
			ProductId: p.ProductId,
			Name:      p.Name,
			Category:  p.Category.String,
			Status:    int32(p.Status),
			Remark:    p.Remark.String,
			CreatedAt: p.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
