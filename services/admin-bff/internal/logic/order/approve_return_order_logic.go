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

type ApproveReturnOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApproveReturnOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApproveReturnOrderLogic {
	return &ApproveReturnOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApproveReturnOrderLogic) ApproveReturnOrder(req *types.ReturnApproveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Order.ApproveReturnOrder(l.ctx, &opb.ApproveReturnOrderReq{
		ReturnNo: req.ReturnNo, Approve: req.Approve, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
