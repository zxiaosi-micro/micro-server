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

type RefundExtensionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundExtensionLogic {
	return &RefundExtensionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefundExtensionLogic) RefundExtension(req *types.ExtensionRefundReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.RefundExtension(l.ctx, &ctpb.RefundExtensionReq{
		ExtensionNo: req.ExtensionNo, Reason: req.Reason,
	})
	return &types.SimpleResp{}, err
}
