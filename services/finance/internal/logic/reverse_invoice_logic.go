package logic

import (
	"context"

	"micro-server/services/finance/internal/svc"
	"micro-server/services/finance/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ReverseInvoiceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReverseInvoiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReverseInvoiceLogic {
	return &ReverseInvoiceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReverseInvoiceLogic) ReverseInvoice(in *pb.ReverseInvoiceReq) (*pb.ReverseInvoiceResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := ReverseInvoiceInternal(l.ctx, l.svcCtx, tid, in.InvoiceNo, in.Reason, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.ReverseInvoiceResp{}, nil
}
