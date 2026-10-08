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

type SettleClaimLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettleClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleClaimLogic {
	return &SettleClaimLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SettleClaimLogic) SettleClaim(req *types.ClaimSettleReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.SettleClaim(l.ctx, &ctpb.SettleClaimReq{
		ClaimNo: req.ClaimNo, SettleType: req.SettleType, Amount: req.Amount,
	})
	return &types.SimpleResp{}, err
}
