package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUserLogic {
	return &DeleteUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteUser 软删：置 deleted_at + 解绑角色 + 踢全部会话 + 失效 auth_cache。
func (l *DeleteUserLogic) DeleteUser(in *pb.DeleteUserReq) (*pb.DeleteUserResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Uid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	if _, err := l.svcCtx.Models.User.FindOne(l.ctx, tid, in.Uid); err != nil {
		return nil, errUserNotFound
	}
	if err := l.svcCtx.Models.User.SoftDelete(l.ctx, tid, in.Uid, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	_ = l.svcCtx.Models.UserRole.DeleteByUser(l.ctx, tid, in.Uid)
	_, _ = l.svcCtx.Sessions.RevokeAll(l.ctx, in.Uid)
	_ = l.svcCtx.Sessions.DeleteAuthCache(l.ctx, in.Uid)
	auditLogin(l.ctx, l.svcCtx.Conn, in.Uid, tid, "auth.user_deleted")
	return &pb.DeleteUserResp{}, nil
}
