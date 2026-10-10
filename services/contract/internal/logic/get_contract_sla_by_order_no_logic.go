// GetContractSlaByOrderNo 按来源订单反查 SLA 绑定（S7 ops 挂计时器用，FR-CTR-007）。
package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContractSlaByOrderNoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContractSlaByOrderNoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContractSlaByOrderNoLogic {
	return &GetContractSlaByOrderNoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetContractSlaByOrderNo order_no → contract → contract_sla → strategy；
// 无合同/无绑定返回 strategy=null（ops 不计时），不报错（链路容错口径）。
func (l *GetContractSlaByOrderNoLogic) GetContractSlaByOrderNo(in *pb.GetContractSlaByOrderNoReq) (*pb.GetContractSlaByOrderNoResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.OrderNo == "" {
		return &pb.GetContractSlaByOrderNoResp{}, nil
	}

	contract, err := l.svcCtx.Models.Contract.FindOneByOrderNo(l.ctx, tid, in.OrderNo)
	if err != nil {
		if err == model.ErrNotFound {
			return &pb.GetContractSlaByOrderNoResp{}, nil
		}
		return nil, err
	}
	bind, err := l.svcCtx.Models.ContractSla.FindOneByContract(l.ctx, tid, contract.ContractId)
	if err != nil {
		if err == model.ErrNotFound {
			return &pb.GetContractSlaByOrderNoResp{ContractNo: contract.ContractNo}, nil
		}
		return nil, err
	}
	strategy, err := l.svcCtx.Models.Sla.FindOneScoped(l.ctx, tid, bind.StrategyId)
	if err != nil {
		if err == model.ErrNotFound {
			return &pb.GetContractSlaByOrderNoResp{ContractNo: contract.ContractNo}, nil
		}
		return nil, err
	}
	return &pb.GetContractSlaByOrderNoResp{
		ContractNo: contract.ContractNo,
		Strategy:   buildSlaStrategyDetail(strategy),
	}, nil
}
