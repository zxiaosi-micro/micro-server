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

type AddShipmentTraceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddShipmentTraceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddShipmentTraceLogic {
	return &AddShipmentTraceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddShipmentTraceLogic) AddShipmentTrace(req *types.TraceAddReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Order.AddShipmentTrace(l.ctx, &opb.AddShipmentTraceReq{
		ShipmentNo: req.ShipmentNo, Node: req.Node, Description: req.Description, TraceTime: req.TraceTime,
	})
	return &types.SimpleResp{}, err
}
