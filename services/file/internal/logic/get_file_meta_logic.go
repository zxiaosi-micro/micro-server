package logic

import (
	"context"
	"database/sql"
	"strconv"

	"micro-server/services/file/internal/model"
	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetFileMetaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFileMetaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFileMetaLogic {
	return &GetFileMetaLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetFileMetaLogic) GetFileMeta(in *pb.GetFileMetaReq) (*pb.GetFileMetaResp, error) {
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
	return &pb.GetFileMetaResp{Meta: metaItem(m)}, nil
}

func metaItem(m *model.FileMeta) *pb.FileMetaItem {
	return &pb.FileMetaItem{
		FileId:      m.FileId,
		Bucket:      m.Bucket,
		OssKey:      m.OssKey,
		Name:        m.Name,
		ContentType: m.ContentType,
		Size:        m.Size,
		BizType:     m.BizType,
		BizId:       bizIdString(m.BizId),
		CreatedAt:   m.CreatedAt.UnixMilli(),
	}
}

// bizIdString NullInt64 → 字符串（0=空）。
func bizIdString(v sql.NullInt64) string {
	if !v.Valid || v.Int64 == 0 {
		return ""
	}
	return strconv.FormatInt(v.Int64, 10)
}
