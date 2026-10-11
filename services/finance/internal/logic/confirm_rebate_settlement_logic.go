package logic

import (
	"context"

	"micro-server/services/finance/internal/model"
	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ConfirmRebateSettlementLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfirmRebateSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmRebateSettlementLogic {
	return &ConfirmRebateSettlementLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConfirmRebateSettlement 状态机推进：CONFIRM（PENDING→CONFIRMED，确认后金额锁定）/ PAID（CONFIRMED→PAID）。
func (l *ConfirmRebateSettlementLogic) ConfirmRebateSettlement(in *pb.ConfirmRebateSettlementReq) (*pb.ConfirmRebateSettlementResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	_, err = l.svcCtx.Models.Rebate.FindOneScoped(l.ctx, tid, in.SettlementId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errRebateNotFound
		}
		return nil, err
	}
	var from, to string
	switch in.Action {
	case "CONFIRM":
		from, to = "PENDING", "CONFIRMED"
	case "PAID":
		from, to = "CONFIRMED", "PAID"
	default:
		return nil, errRebateActionBad
	}
	n, err := l.svcCtx.Models.Rebate.CasStatus(l.ctx, tid, in.SettlementId, from, to)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errRebateStatus
	}
	logx.WithContext(l.ctx).Infof("rebate settlement %d %s→%s by=%d remark=%s", in.SettlementId, from, to, ctxkit.UID(l.ctx), in.Remark)
	return &pb.ConfirmRebateSettlementResp{}, nil
}
