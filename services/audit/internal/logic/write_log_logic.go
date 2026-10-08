package logic

import (
	"context"
	"database/sql"

	"micro-server/services/audit/internal/model"
	"micro-server/services/audit/internal/svc"
	"micro-server/services/audit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type WriteLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewWriteLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WriteLogLogic {
	return &WriteLogLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// WriteLog 追加操作审计（禁改删；写入开关 confcenter 应急熔断用）。
func (l *WriteLogLogic) WriteLog(in *pb.WriteLogReq) (*pb.WriteLogResp, error) {
	if in.Action == "" {
		return nil, errActionRequired
	}
	if !validJSON(in.BeforeJson) || !validJSON(in.AfterJson) || !validJSON(in.DetailJson) {
		return nil, errcode.ErrBadRequest.WithMsg("json 字段不合法")
	}
	tid, err := auditTenant(l.ctx, in.TenantId)
	if err != nil {
		return nil, err
	}

	lid := l.svcCtx.Snowflake.MustNextID()
	_, err = l.svcCtx.Models.AuditLog.Insert(l.ctx, &model.AuditLog{
		LogId:      lid,
		TraceId:    in.TraceId,
		Uid:        in.Uid,
		Action:     in.Action,
		TargetType: toNullString(in.TargetType),
		TargetId:   toNullString(in.TargetId),
		BeforeJson: toNullString(in.BeforeJson),
		AfterJson:  toNullString(in.AfterJson),
		Result:     defaultStr(in.Result, "OK"),
		Client:     toNullString(in.Client),
		OnBehalfOf: toNullInt64(in.OnBehalfOf),
		DetailJson: toNullString(in.DetailJson),
		TenantId:   tid,
	})
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.WriteLogResp{LogId: lid}, nil
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
