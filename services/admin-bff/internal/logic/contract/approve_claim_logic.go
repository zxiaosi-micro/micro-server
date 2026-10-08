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

type ApproveClaimLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApproveClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveClaimLogic {
	return &ApproveClaimLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApproveClaimLogic) ApproveClaim(req *types.ClaimApproveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.ApproveClaim(l.ctx, &ctpb.ApproveClaimReq{
		ClaimNo: req.ClaimNo, Approve: req.Approve, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
