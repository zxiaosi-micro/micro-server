package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type TransferExtensionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewTransferExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TransferExtensionLogic {
	return &TransferExtensionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *TransferExtensionLogic) TransferExtension(in *pb.TransferExtensionReq) (*pb.TransferExtensionResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := TransferExtensionInternal(l.ctx, l.svcCtx, tid, in.ExtensionNo, in.ToTargetType, in.ToTargetId, in.ToTargetKey); err != nil {
		return nil, err
	}
	return &pb.TransferExtensionResp{}, nil
}
