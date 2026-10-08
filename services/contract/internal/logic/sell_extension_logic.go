package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type SellExtensionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSellExtensionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SellExtensionLogic {
	return &SellExtensionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SellExtensionLogic) SellExtension(in *pb.SellExtensionReq) (*pb.SellExtensionResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	extId, extNo, err := SellExtensionInternal(l.ctx, l.svcCtx, tid, in.BaseWarrantyId, in.Months, in.OrderNo, in.Amount, ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	return &pb.SellExtensionResp{ExtensionId: extId, ExtensionNo: extNo}, nil
}
