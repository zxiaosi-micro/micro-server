package logic

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"micro-server/services/file/internal/model"
	"micro-server/services/file/internal/svc"
	"micro-server/services/file/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type PutFileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPutFileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PutFileLogic {
	return &PutFileLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// PutFile 上传：对象入双桶（biz_type 路由）+ 元数据落库（oss_key 规范 02 §6.5）。
func (l *PutFileLogic) PutFile(in *pb.PutFileReq) (*pb.PutFileResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Name == "" || len(in.Data) == 0 {
		return nil, errcode.ErrBadRequest.WithMsg("name/data 必填")
	}
	if in.BizType == "" {
		in.BizType = "common"
	}
	contentType := in.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	bucket := bucketFor(l.svcCtx, in.BizType)
	key := buildOssKey(tid, in.BizType, in.Name)
	if err := putObject(l.ctx, l.svcCtx, bucket, key, contentType, in.Data); err != nil {
		return nil, err
	}

	sum := sha256.Sum256(in.Data)
	fid := l.svcCtx.Snowflake.MustNextID()
	op := opUID(l.ctx)
	bizId := parseInt64Null(in.BizId)
	_, err = l.svcCtx.Models.FileMeta.Insert(l.ctx, &model.FileMeta{
		FileId:      fid,
		Bucket:      bucket,
		OssKey:      key,
		Name:        in.Name,
		ContentType: contentType,
		Size:        int64(len(in.Data)),
		BizType:     in.BizType,
		BizId:       bizId,
		Sha256:      toNullString(hex.EncodeToString(sum[:])),
		TenantId:    tid,
		CreatedBy:   toNullInt64(op),
		UpdatedBy:   toNullInt64(op),
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.PutFileResp{FileId: fid, OssKey: key, Bucket: bucket}, nil
}

func parseInt64Null(s string) sql.NullInt64 {
	if s == "" {
		return sql.NullInt64{}
	}
	v, err := parseInt64(s)
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: v, Valid: true}
}
