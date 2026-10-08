package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListClaimLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListClaimLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListClaimLogic {
	return &ListClaimLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListClaim 索赔列表（FR-CTR-008）。
func (l *ListClaimLogic) ListClaim(in *pb.ListClaimReq) (*pb.ListClaimResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Claim.ListPage(l.ctx, tid, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListClaimResp{Total: total}
	for _, c := range list {
		resp.List = append(resp.List, buildClaimDetail(c))
	}
	return resp, nil
}
