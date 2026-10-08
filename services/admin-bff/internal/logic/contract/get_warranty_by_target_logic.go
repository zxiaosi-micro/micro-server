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

type GetWarrantyByTargetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetWarrantyByTargetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWarrantyByTargetLogic {
	return &GetWarrantyByTargetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetWarrantyByTargetLogic) GetWarrantyByTarget(req *types.WarrantyTargetReq) (resp *types.WarrantyTargetResp, err error) {
	respOut, err := l.svcCtx.Contract.GetWarrantyByTarget(l.ctx, &ctpb.GetWarrantyByTargetReq{
		TargetType: req.TargetType, TargetId: parseI64(req.TargetId), Sn: req.Sn,
	})
	if err != nil {
		return nil, err
	}
	out := &types.WarrantyTargetResp{}
	for _, w := range respOut.List {
		out.List = append(out.List, *warrantyView(w))
	}
	return out, nil
}
