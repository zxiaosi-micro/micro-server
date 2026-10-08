// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package contract

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListClaimsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListClaimsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListClaimsLogic {
	return &ListClaimsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListClaimsLogic) ListClaims(req *types.ClaimListReq) (resp *types.ClaimListResp, err error) {
	respOut, err := l.svcCtx.Contract.ListClaim(l.ctx, &ctpb.ListClaimReq{
		Status: req.Status, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.ClaimListResp{Total: int(respOut.Total)}
	for _, c := range respOut.List {
		out.List = append(out.List, types.ClaimView{
			ClaimNo: c.ClaimNo, WarrantyId: strconv.FormatInt(c.WarrantyId, 10),
			WarrantyNo: c.WarrantyNo, Type: c.Type, Description: c.Description,
			Status: c.Status, SettleType: c.SettleType, Amount: c.Amount, CreatedAt: c.CreatedAt,
		})
	}
	return out, nil
}
