package logic

import (
	"context"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ArchiveContractLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewArchiveContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveContractLogic {
	return &ArchiveContractLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ArchiveContractLogic) ArchiveContract(in *pb.ArchiveContractReq) (*pb.ArchiveContractResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if err := ArchiveContractInternal(l.ctx, l.svcCtx, tid, in.ContractNo, in.Action, in.Remark, ctxkit.UID(l.ctx)); err != nil {
		return nil, err
	}
	return &pb.ArchiveContractResp{}, nil
}
