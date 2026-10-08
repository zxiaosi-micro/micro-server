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

type UploadContractFileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadContractFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadContractFileLogic {
	return &UploadContractFileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadContractFileLogic) UploadContractFile(req *types.ContractFileUploadReq) (resp *types.ContractFileUploadResp, err error) {
	respOut, err := l.svcCtx.Contract.UploadContractFile(l.ctx, &ctpb.UploadContractFileReq{
		ContractNo: req.ContractNo, FileId: req.FileId, FileName: req.FileName,
		SignPartyName: req.SignPartyName, SignPartyType: req.SignPartyType, Remark: req.Remark,
	})
	if err != nil {
		return nil, err
	}
	return &types.ContractFileUploadResp{Version: int(respOut.Version)}, nil
}
