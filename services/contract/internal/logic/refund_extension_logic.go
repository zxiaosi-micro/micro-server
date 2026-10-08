package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type RefundExtensionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundExtensionLogic {
	return &RefundExtensionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RefundExtensionLogic) RefundExtension(in *pb.RefundExtensionReq) (*pb.RefundExtensionResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := RefundExtensionInternal(l.ctx, l.svcCtx, tid, in.ExtensionNo, in.Reason, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.RefundExtensionResp{}, nil
}
