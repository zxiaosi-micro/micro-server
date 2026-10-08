package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type CreateFromOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateFromOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateFromOrderLogic {
	return &CreateFromOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateFromOrderLogic) CreateFromOrder(in *pb.CreateFromOrderReq) (*pb.CreateFromOrderResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.OrderNo == "" {
		return nil, errOrderNoRequired
	}
	contractId, contractNo, warrantyCount, err := CreateFromOrderInternal(l.ctx, l.svcCtx, tid,
		in.OrderNo, in.OrderType, in.BuyerPartyId, in.BuyerName, in.Amount, in.ItemsJson, in.ContractTemplateId, ctxkit.UID(l.ctx))
	if err != nil {
		return nil, err
	}
	return &pb.CreateFromOrderResp{ContractId: contractId, ContractNo: contractNo, WarrantyCount: int64(warrantyCount)}, nil
}
