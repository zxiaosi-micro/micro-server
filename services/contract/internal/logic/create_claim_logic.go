package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateClaimLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateClaimLogic {
	return &CreateClaimLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateClaimLogic) CreateClaim(in *pb.CreateClaimReq) (*pb.CreateClaimResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	claimId, claimNo, err := CreateClaimInternal(l.ctx, l.svcCtx, tid, in.WarrantyId, in.Type, in.Description, ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	return &pb.CreateClaimResp{ClaimId: claimId, ClaimNo: claimNo}, nil
}
