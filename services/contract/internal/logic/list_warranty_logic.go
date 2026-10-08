package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWarrantyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWarrantyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWarrantyLogic {
	return &ListWarrantyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWarrantyLogic) ListWarranty(in *pb.ListWarrantyReq) (*pb.ListWarrantyResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Warranty.ListPage(l.ctx, tid, in.Status, in.Level, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListWarrantyResp{Total: total}
	for _, w := range list {
		resp.List = append(resp.List, buildWarrantyItem(w))
	}
	return resp, nil
}
