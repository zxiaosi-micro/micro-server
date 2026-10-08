package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ApproveClaimLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApproveClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveClaimLogic {
	return &ApproveClaimLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApproveClaimLogic) ApproveClaim(in *pb.ApproveClaimReq) (*pb.ApproveClaimResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := ApproveClaimInternal(l.ctx, l.svcCtx, tid, in.ClaimNo, in.Approve, in.Remark, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.ApproveClaimResp{}, nil
}
