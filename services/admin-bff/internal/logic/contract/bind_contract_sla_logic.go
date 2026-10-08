// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package contract

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindContractSlaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindContractSlaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindContractSlaLogic {
	return &BindContractSlaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BindContractSlaLogic) BindContractSla(req *types.SlaBindReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.BindContractSla(l.ctx, &ctpb.BindContractSlaReq{
		ContractNo: req.ContractNo, StrategyId: parseI64(req.StrategyId),
	})
	return &types.SimpleResp{}, err
}
