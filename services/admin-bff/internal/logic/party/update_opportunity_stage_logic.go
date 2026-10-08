// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateOpportunityStageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商机阶段推进(perm: party:opportunity:update)
func NewUpdateOpportunityStageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateOpportunityStageLogic {
	return &UpdateOpportunityStageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateOpportunityStageLogic) UpdateOpportunityStage(req *types.OpportunityStageReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.UpdateOpportunityStage(l.ctx, &partyPb.UpdateOpportunityStageReq{
		OpportunityId: parseID(req.Id), Stage: req.Stage,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
