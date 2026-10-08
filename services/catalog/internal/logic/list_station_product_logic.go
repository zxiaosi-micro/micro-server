package logic

import (
	"context"

	"micro-server/services/catalog/internal/svc"
	"micro-server/services/catalog/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListStationProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListStationProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListStationProductLogic {
	return &ListStationProductLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListStationProductLogic) ListStationProduct(in *pb.ListStationProductReq) (*pb.ListStationProductResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.StationProduct.FindPage(l.ctx, tid, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListStationProductResp{Total: total}
	for _, sp := range list {
		resp.List = append(resp.List, &pb.StationProductItem{
			StationProductId: sp.StationProductId,
			Name:             sp.Name,
			Remark:           sp.Remark.String,
			CreatedAt:        sp.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
