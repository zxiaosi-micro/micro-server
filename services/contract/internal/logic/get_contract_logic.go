package logic

import (
	"context"

	"micro-server/services/contract/internal/model"
	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetContractLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetContractLogic {
	return &GetContractLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetContractLogic) GetContract(in *pb.GetContractReq) (*pb.GetContractResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	var c *model.Contract
	if in.ContractId > 0 {
		c, err = l.svcCtx.Models.Contract.FindOneScoped(l.ctx, tid, in.ContractId)
	} else {
		c, err = l.svcCtx.Models.Contract.FindOneByNo(l.ctx, tid, in.ContractNo)
	}
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errContractNotFound
		}
		return nil, err
	}
	files, err := l.svcCtx.Models.File.ListByContract(l.ctx, tid, c.ContractId)
	if err != nil {
		return nil, err
	}
	targets, err := l.svcCtx.Models.Target.ListByContract(l.ctx, tid, c.ContractId)
	if err != nil {
		return nil, err
	}
	return &pb.GetContractResp{Contract: buildContractDetail(c, files, targets)}, nil
}
