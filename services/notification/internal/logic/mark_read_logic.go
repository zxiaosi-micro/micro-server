package logic

import (
	"context"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type MarkReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkReadLogic {
	return &MarkReadLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// MarkRead 标记已读（SQL 条件内校验本人，越权返回不存在语义）。
func (l *MarkReadLogic) MarkRead(in *pb.MarkReadReq) (*pb.MarkReadResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.MessageId <= 0 || in.UserId <= 0 {
		return nil, errUserRequired
	}
	if err := l.svcCtx.Models.Message.MarkRead(l.ctx, tid, in.MessageId, in.UserId); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.MarkReadResp{}, nil
}
