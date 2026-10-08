package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type SettleClaimLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSettleClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettleClaimLogic {
	return &SettleClaimLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SettleClaimLogic) SettleClaim(in *pb.SettleClaimReq) (*pb.SettleClaimResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := SettleClaimInternal(l.ctx, l.svcCtx, tid, in.ClaimNo, in.SettleType, in.Amount, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.SettleClaimResp{}, nil
}
