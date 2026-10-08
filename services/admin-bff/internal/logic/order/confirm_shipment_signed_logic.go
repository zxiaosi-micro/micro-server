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

type ConfirmShipmentSignedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConfirmShipmentSignedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfirmShipmentSignedLogic {
	return &ConfirmShipmentSignedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfirmShipmentSignedLogic) ConfirmShipmentSigned(req *types.ShipmentSignReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Order.ConfirmShipmentSigned(l.ctx, &opb.ConfirmShipmentSignedReq{
		ShipmentNo: req.ShipmentNo, SignedBy: req.SignedBy,
	})
	return &types.SimpleResp{}, err
}
