package logic

import (
	"context"
	"database/sql"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateOpportunityStageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateOpportunityStageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateOpportunityStageLogic {
	return &UpdateOpportunityStageLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// UpdateOpportunityStage 看板阶段推进（stage VARCHAR 枚举：加枚举零 DDL）。
func (l *UpdateOpportunityStageLogic) UpdateOpportunityStage(in *pb.UpdateOpportunityStageReq) (*pb.UpdateOpportunityStageResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	switch in.Stage {
	case "NEW", "CONTACTING", "PROPOSAL", "NEGOTIATION", "WON", "LOST":
	default:
		return nil, errStageBad
	}
	if _, err := l.svcCtx.Models.Opportunity.FindOne(l.ctx, tid, in.OpportunityId); err != nil {
		return nil, partyErr(err)
	}
	if err := l.svcCtx.Models.Opportunity.UpdateStage(l.ctx, tid, in.OpportunityId, in.Stage, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UpdateOpportunityStageResp{}, nil
}

var _ = sql.NullString{}
