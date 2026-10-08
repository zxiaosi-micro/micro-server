package logic

import (
	"context"

	"micro-server/services/file/internal/model"
	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFileLogic {
	return &DeleteFileLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// DeleteFile 软删元数据（对象保留，物理回收属运维流程）。
func (l *DeleteFileLogic) DeleteFile(in *pb.DeleteFileReq) (*pb.DeleteFileResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := l.svcCtx.Models.FileMeta.FindOne(l.ctx, tid, in.FileId); err != nil {
		if err == model.ErrNotFound {
			return nil, errFileNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	if err := l.svcCtx.Models.FileMeta.SoftDelete(l.ctx, tid, in.FileId, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.DeleteFileResp{}, nil
}
