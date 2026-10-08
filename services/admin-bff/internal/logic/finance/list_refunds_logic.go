// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package finance

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	finpb "micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListRefundsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListRefundsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListRefundsLogic {
	return &ListRefundsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListRefundsLogic) ListRefunds(req *types.RefundListReq) (resp *types.RefundListResp, err error) {
	respOut, err := l.svcCtx.Finance.ListRefund(l.ctx, &finpb.ListRefundReq{
		Keyword: req.Keyword, Status: req.Status, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.RefundListResp{Total: int(respOut.Total)}
	for _, r := range respOut.List {
		out.List = append(out.List, types.RefundView{
			RefundNo: r.RefundNo, PaymentNo: r.PaymentNo, OrderNo: r.OrderNo,
			ReturnNo: r.ReturnNo, Amount: r.Amount, Channel: r.Channel, Status: r.Status,
			ChannelRefundId: r.ChannelRefundId, Reason: r.Reason, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
