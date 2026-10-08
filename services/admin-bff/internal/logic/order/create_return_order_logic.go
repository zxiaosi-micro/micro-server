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

type CreateReturnOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateReturnOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReturnOrderLogic {
	return &CreateReturnOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateReturnOrderLogic) CreateReturnOrder(req *types.ReturnCreateReq) (resp *types.ReturnCreateResp, err error) {
	items := make([]*opb.ReturnItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, &opb.ReturnItemInput{
			SkuId: parseI64(it.SkuId), Qty: int32(it.Qty), Sn: it.Sn, Reason: it.Reason,
		})
	}
	respOut, err := l.svcCtx.Order.CreateReturnOrder(l.ctx, &opb.CreateReturnOrderReq{
		OrderNo: req.OrderNo, Items: items, Reason: req.Reason,
	})
	if err != nil {
		return nil, err
	}
	return &types.ReturnCreateResp{
		ReturnId: strconv.FormatInt(respOut.ReturnId, 10), ReturnNo: respOut.ReturnNo,
	}, nil
}
