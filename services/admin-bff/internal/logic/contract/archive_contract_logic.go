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

type ArchiveContractLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewArchiveContractLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ArchiveContractLogic {
	return &ArchiveContractLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ArchiveContractLogic) ArchiveContract(req *types.ContractArchiveReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Contract.ArchiveContract(l.ctx, &ctpb.ArchiveContractReq{
		ContractNo: req.ContractNo, Action: req.Action, Remark: req.Remark,
	})
	return &types.SimpleResp{}, err
}
