package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateUserStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserStatusLogic {
	return &UpdateUserStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateUserStatus 禁用即时踢全部会话 + 失效 auth_cache（02 §9.5）。
func (l *UpdateUserStatusLogic) UpdateUserStatus(in *pb.UpdateUserStatusReq) (*pb.UpdateUserStatusResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Uid <= 0 || (in.Status != 1 && in.Status != 2) {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填；status 仅 1/2")
	}
	if _, err := l.svcCtx.Models.User.FindOne(l.ctx, tid, in.Uid); err != nil {
		return nil, errUserNotFound
	}
	if err := l.svcCtx.Models.User.UpdateStatus(l.ctx, tid, in.Uid, int64(in.Status), opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if in.Status == 2 {
		if n, kerr := l.svcCtx.Sessions.RevokeAll(l.ctx, in.Uid); kerr != nil {
			l.Errorf("禁用踢会话失败 uid=%d: %v", in.Uid, kerr)
		} else {
			l.Infof("禁用用户 uid=%d，注销会话 %d 个", in.Uid, n)
		}
		if derr := l.svcCtx.Sessions.DeleteAuthCache(l.ctx, in.Uid); derr != nil {
			l.Errorf("auth_cache 失效失败 uid=%d: %v", in.Uid, derr)
		}
		auditLogin(l.ctx, l.svcCtx.Conn, in.Uid, tid, "auth.user_disabled")
	}
	return &pb.UpdateUserStatusResp{}, nil
}
