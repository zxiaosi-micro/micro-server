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

type TransferExtensionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTransferExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferExtensionLogic {
	return &TransferExtensionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TransferExtensionLogic) TransferExtension(req *types.ExtensionTransferReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.TransferExtension(l.ctx, &ctpb.TransferExtensionReq{
		ExtensionNo: req.ExtensionNo, ToTargetType: req.ToTargetType,
		ToTargetId: parseI64(req.ToTargetId), ToTargetKey: req.ToTargetKey,
	})
	return &types.SimpleResp{}, err
}
