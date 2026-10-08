package logic

import (
	"context"

	"micro-server/services/file/internal/model"
	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetFileDataLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFileDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFileDataLogic {
	return &GetFileDataLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetFileData 校验令牌并回源对象内容（BFF 代理下载的数据面；
// 令牌内含租户声明，租户过滤不破口）。
func (l *GetFileDataLogic) GetFileData(in *pb.GetFileDataReq) (*pb.GetFileDataResp, error) {
	claims, err := verifyToken(l.svcCtx.SignKey(), in.Token)
	if err != nil {
		return nil, err
	}
	m, err := l.svcCtx.Models.FileMeta.FindOne(l.ctx, claims.TenantID, claims.FileID)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errFileNotFound
		}
		return nil, errcode.Internal.WithCause(err)
	}
	data, err := getObject(l.ctx, l.svcCtx, m.Bucket, m.OssKey)
	if err != nil {
		return nil, err
	}
	return &pb.GetFileDataResp{Data: data, FileName: m.Name, ContentType: m.ContentType}, nil
}
