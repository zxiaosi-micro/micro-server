package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertRuleLogic {
	return &DeleteAlertRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteAlertRuleLogic) DeleteAlertRule(in *pb.DeleteAlertRuleReq) (*pb.DeleteAlertRuleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Models.AlertRule.SoftDelete(l.ctx, tid, in.RuleId); err != nil {
		return nil, err
	}
	return &pb.DeleteAlertRuleResp{}, nil
}
