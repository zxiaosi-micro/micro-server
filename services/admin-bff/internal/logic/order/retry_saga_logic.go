// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package order

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetrySagaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetrySagaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetrySagaLogic {
	return &RetrySagaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetrySagaLogic) RetrySaga(req *types.SagaRetryReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Order.RetrySaga(l.ctx, &opb.RetrySagaReq{SagaId: req.Id, Remark: req.Remark})
	return &types.SimpleResp{}, err
}
