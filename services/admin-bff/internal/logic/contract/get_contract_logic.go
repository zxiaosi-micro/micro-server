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

type GetContractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContractLogic {
	return &GetContractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetContractLogic) GetContract(req *types.ContractNoPath) (resp *types.ContractView, err error) {
	respOut, err := l.svcCtx.Contract.GetContract(l.ctx, &ctpb.GetContractReq{ContractNo: req.ContractNo})
	if err != nil {
		return nil, err
	}
	return contractView(respOut.Contract), nil
}
