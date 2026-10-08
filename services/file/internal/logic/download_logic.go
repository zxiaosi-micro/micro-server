package logic

import (
	"context"
	"time"

	"micro-server/services/file/internal/model"
	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetDownloadTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDownloadTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDownloadTokenLogic {
	return &GetDownloadTokenLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetDownloadToken 签发临时下载令牌（HMAC，02 §6.5）。
func (l *GetDownloadTokenLogic) GetDownloadToken(in *pb.GetDownloadTokenReq) (*pb.GetDownloadTokenResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	m, err := l.svcCtx.Models.FileMeta.FindOne(l.ctx, tid, in.FileId)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errFileNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	expTTL := tokenExpSec(l.svcCtx)
	if in.ExpireSec > 0 && in.ExpireSec < 3600 {
		expTTL = time.Duration(in.ExpireSec) * time.Second
	}
	token := signToken(l.svcCtx.SignKey(), tid, m.FileId, time.Now().Add(expTTL))
	return &pb.GetDownloadTokenResp{Token: token, FileName: m.Name, ContentType: m.ContentType}, nil
}
