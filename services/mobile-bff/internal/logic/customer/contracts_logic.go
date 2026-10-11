// contracts_logic.go · 我的合同（ListContract buyer_party_id 过滤，S9-01 增量契约）。

package customer

import (
	"context"

	ctpb "micro-server/services/contract/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ContractsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewContractsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ContractsLogic {
	return &ContractsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ContractsLogic) Contracts(req *types.ContractListReq) (resp *types.ContractListResp, err error) {
	partyId, err := requireParty(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}
	out, err := l.svcCtx.Contract.ListContract(l.ctx, &ctpb.ListContractReq{
		BuyerPartyId: partyId,
		Page:         req.Page,
		Size:         req.Size,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.ContractListResp{Total: out.Total, List: []types.ContractView{}}
	for _, c := range out.List {
		resp.List = append(resp.List, pbContract(c))
	}
	return resp, nil
}
