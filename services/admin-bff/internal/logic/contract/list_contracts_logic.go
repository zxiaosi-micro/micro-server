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

type ListContractsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListContractsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContractsLogic {
	return &ListContractsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListContractsLogic) ListContracts(req *types.ContractListReq) (resp *types.ContractListResp, err error) {
	respOut, err := l.svcCtx.Contract.ListContract(l.ctx, &ctpb.ListContractReq{
		Keyword: req.Keyword, Type: req.Type, Status: req.Status,
		Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	out := &types.ContractListResp{Total: int(respOut.Total)}
	for _, c := range respOut.List {
		out.List = append(out.List, *contractView(c))
	}
	return out, nil
}
