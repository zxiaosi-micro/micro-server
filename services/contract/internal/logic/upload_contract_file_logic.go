package logic

import (
	"context"
	"fmt"

	"micro-server/services/contract/internal/svc"
	"micro-server/services/contract/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type UploadContractFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUploadContractFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadContractFileLogic {
	return &UploadContractFileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UploadContractFileLogic) UploadContractFile(in *pb.UploadContractFileReq) (*pb.UploadContractFileResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	// 上传人姓名展示位：基线取 uid 字符串（真实姓名经 party 域反查为 L2 演进位）
	uploaderName := ""
	if uid := ctxkit.UID(l.ctx); uid > 0 {
		uploaderName = fmt.Sprintf("u%d", uid)
	}
	version, err := UploadContractFileInternal(l.ctx, l.svcCtx, tid, in.ContractNo, in.FileId, in.FileName,
		in.SignPartyName, in.SignPartyType, in.Remark, ctxkit.UID(l.ctx), uploaderName)
	if err != nil {
		return nil, err
	}
	return &pb.UploadContractFileResp{Version: int32(version)}, nil
}
