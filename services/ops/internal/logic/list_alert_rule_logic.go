package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAlertRuleLogic {
	return &ListAlertRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListAlertRuleLogic) ListAlertRule(in *pb.ListAlertRuleReq) (*pb.ListAlertRuleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	enabled := in.Enabled
	if enabled == 0 {
		enabled = 1
	}
	list, total, err := l.svcCtx.Models.AlertRule.ListPage(l.ctx, tid, in.Metric, in.Level, enabled, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListAlertRuleResp{Total: total}
	for _, r := range list {
		resp.List = append(resp.List, pbRule(r))
	}
	return resp, nil
}
