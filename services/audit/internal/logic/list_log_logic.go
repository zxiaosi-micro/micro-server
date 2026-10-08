package logic

import (
	"context"

	"micro-server/services/audit/internal/svc"
	"micro-server/services/audit/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListLogLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogLogic {
	return &ListLogLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListLogLogic) ListLog(in *pb.ListLogReq) (*pb.ListLogResp, error) {
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.AuditLog.FindPage(l.ctx, in.Action, in.Uid, in.TargetType, in.TargetId, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListLogResp{Total: total}
	for _, r := range list {
		resp.List = append(resp.List, &pb.AuditLogItem{
			LogId:      r.LogId,
			TraceId:    r.TraceId,
			Uid:        r.Uid,
			Action:     r.Action,
			TargetType: r.TargetType.String,
			TargetId:   r.TargetId.String,
			BeforeJson: r.BeforeJson.String,
			AfterJson:  r.AfterJson.String,
			Result:     r.Result,
			Client:     r.Client.String,
			OnBehalfOf: r.OnBehalfOf.Int64,
			CreatedAt:  r.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
