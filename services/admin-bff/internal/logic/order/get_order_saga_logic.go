// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package order

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	opb "micro-server/services/order/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderSagaLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetOrderSagaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderSagaLogic {
	return &GetOrderSagaLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetOrderSagaLogic) GetOrderSaga(req *types.OrderNoPath) (resp *types.SagaResp, err error) {
	respOut, err := l.svcCtx.Order.GetSaga(l.ctx, &opb.GetSagaReq{OrderNo: req.OrderNo})
	if err != nil {
		return nil, err
	}
	sg := respOut.Saga
	out := &types.SagaResp{Saga: types.SagaView{
		SagaId: strconv.FormatInt(sg.SagaId, 10), OrderNo: sg.OrderNo, OrderType: sg.OrderType,
		CurrentStep: int(sg.CurrentStep), Status: sg.Status, RetryCount: int(sg.RetryCount),
		NextRetryAt: sg.NextRetryAt, LastError: sg.LastError, UpdatedAt: sg.UpdatedAt,
	}}
	for _, st := range sg.Steps {
		out.Saga.Steps = append(out.Saga.Steps, types.SagaStepView{
			Step: int(st.Step), Name: st.Name, Status: st.Status, IdemKey: st.IdemKey,
		})
	}
	return out, nil
}
