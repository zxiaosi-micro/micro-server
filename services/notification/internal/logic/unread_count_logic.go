package logic

import (
	"context"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UnreadCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnreadCountLogic {
	return &UnreadCountLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UnreadCountLogic) UnreadCount(in *pb.UnreadCountReq) (*pb.UnreadCountResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 {
		return nil, errUserRequired
	}
	n, err := l.svcCtx.Models.Message.CountUnread(l.ctx, tid, in.UserId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.UnreadCountResp{Count: n}, nil
}
