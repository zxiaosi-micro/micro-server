package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfirmRebateSettlementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 返利结算确认/打款(PENDING→CONFIRMED→PAID)
func NewConfirmRebateSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmRebateSettlementLogic {
	return &ConfirmRebateSettlementLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConfirmRebateSettlementLogic) ConfirmRebateSettlement(req *types.RebateConfirmReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Finance.ConfirmRebateSettlement(l.ctx, &finpb.ConfirmRebateSettlementReq{
		SettlementId: parseI64(req.Id), Action: req.Action, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
