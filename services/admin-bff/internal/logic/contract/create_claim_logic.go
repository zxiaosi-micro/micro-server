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

type CreateClaimLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateClaimLogic {
	return &CreateClaimLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateClaimLogic) CreateClaim(req *types.ClaimCreateReq) (resp *types.ClaimCreateResp, err error) {
	respOut, err := l.svcCtx.Contract.CreateClaim(l.ctx, &ctpb.CreateClaimReq{
		WarrantyId: parseI64(req.WarrantyId), Type: req.Type, Description: req.Description,
	})
	if err != nil {
		return nil, err
	}
	return &types.ClaimCreateResp{
		ClaimId: strconv.FormatInt(respOut.ClaimId, 10), ClaimNo: respOut.ClaimNo,
	}, nil
}
