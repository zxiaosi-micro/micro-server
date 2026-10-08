// Code scaffolded by goctl. Safe to edit. Implementation: S5-02~04.
// goctl 1.10.2

package contract

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	ctpb "micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SellExtensionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSellExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SellExtensionLogic {
	return &SellExtensionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SellExtensionLogic) SellExtension(req *types.ExtensionSellReq) (resp *types.ExtensionSellResp, err error) {
	respOut, err := l.svcCtx.Contract.SellExtension(l.ctx, &ctpb.SellExtensionReq{
		BaseWarrantyId: parseI64(req.BaseWarrantyId), Months: int32(req.Months),
		OrderNo: req.OrderNo, Amount: req.Amount,
	})
	if err != nil {
		return nil, err
	}
	return &types.ExtensionSellResp{
		ExtensionId: strconv.FormatInt(respOut.ExtensionId, 10), ExtensionNo: respOut.ExtensionNo,
	}, nil
}
