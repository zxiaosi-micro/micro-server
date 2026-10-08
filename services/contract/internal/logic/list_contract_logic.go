package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListContractLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContractLogic {
	return &ListContractLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListContractLogic) ListContract(in *pb.ListContractReq) (*pb.ListContractResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Contract.ListPage(l.ctx, tid, in.Keyword, in.Type, in.Status, page, size)
	if err != nil {
		return nil, err
	}
	resp := &pb.ListContractResp{Total: total}
	for _, c := range list {
		files, err := l.svcCtx.Models.File.ListByContract(l.ctx, tid, c.ContractId)
		if err != nil {
			return nil, err
		}
		targets, err := l.svcCtx.Models.Target.ListByContract(l.ctx, tid, c.ContractId)
		if err != nil {
			return nil, err
		}
		resp.List = append(resp.List, buildContractDetail(c, files, targets))
	}
	return resp, nil
}
