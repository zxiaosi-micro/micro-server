package logic

import (
	"context"

	"micro-server/services/ops/internal/svc"
	"micro-server/services/ops/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"micro-server/services/ops/internal/model"
)

type CreateAlertRuleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAlertRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAlertRuleLogic {
	return &CreateAlertRuleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ---- 告警规则（FR-OPS-001）----
func (l *CreateAlertRuleLogic) CreateAlertRule(in *pb.CreateAlertRuleReq) (*pb.CreateAlertRuleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Metric == "" || !validComparator(in.Comparator) {
		return nil, errRuleBad
	}
	if in.Level == "" {
		in.Level = "P2"
	}
	if in.SuppressWindowSec <= 0 {
		in.SuppressWindowSec = 300
	}
	if _, err := l.svcCtx.Models.AlertRule.FindOneByCodeScoped(l.ctx, tid, in.Code); err == nil {
		return nil, errRuleCodeDup
	} else if err != model.ErrNotFound {
		return nil, err
	}
	row := &model.AlertRule{
		RuleId:            l.svcCtx.Snowflake.MustNextID(),
		Code:              in.Code,
		Name:              in.Name,
		Metric:            in.Metric,
		Comparator:        in.Comparator,
		Threshold:         in.Threshold,
		DurationSec:       int64(in.DurationSec),
		Level:             in.Level,
		SuppressWindowSec: int64(in.SuppressWindowSec),
		Scope:             nullString(in.Scope),
		Enabled:           1,
		TenantId:          tid,
		CreatedBy:         nullInt64(ctxkit.UID(l.ctx)),
	}
	if err := l.svcCtx.Models.AlertRule.InsertTx(l.ctx, l.svcCtx.Conn, row); err != nil {
		return nil, err
	}
	return &pb.CreateAlertRuleResp{RuleId: row.RuleId}, nil
}
