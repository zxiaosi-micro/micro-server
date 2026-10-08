// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package contract

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWarrantiesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListWarrantiesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWarrantiesLogic {
	return &ListWarrantiesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListWarrantiesLogic) ListWarranties(req *types.WarrantyListReq) (resp *types.WarrantyListResp, err error) {
	respOut, err := l.svcCtx.Contract.ListWarranty(l.ctx, &ctpb.ListWarrantyReq{
		Status: req.Status, Level: req.Level, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.WarrantyListResp{Total: int(respOut.Total)}
	for _, w := range respOut.List {
		out.List = append(out.List, *warrantyView(w))
	}
	return out, nil
}
