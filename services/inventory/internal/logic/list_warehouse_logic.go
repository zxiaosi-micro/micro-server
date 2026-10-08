package logic

import (
	"context"

	"micro-server/services/inventory/internal/svc"
	"micro-server/services/inventory/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListWarehouseLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWarehouseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWarehouseLogic {
	return &ListWarehouseLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListWarehouseLogic) ListWarehouse(in *pb.ListWarehouseReq) (*pb.ListWarehouseResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Warehouse.FindPage(l.ctx, tid, in.Keyword, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListWarehouseResp{Total: total}
	for _, w := range list {
		resp.List = append(resp.List, &pb.WarehouseItem{
			WarehouseId: w.WarehouseId,
			Code:        w.Code,
			Name:        w.Name,
			Address:     w.Address.String,
			Status:      int32(w.Status),
			CreatedAt:   w.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
