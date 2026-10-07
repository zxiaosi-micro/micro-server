package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ResetPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetPasswordLogic {
	return &ResetPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResetPassword 管理端重置密码：argon2id 落库 + 全端注销（旧会话凭证不再可信）+ 审计。
func (l *ResetPasswordLogic) ResetPassword(in *pb.ResetPasswordReq) (*pb.ResetPasswordResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.Uid <= 0 || len(in.NewPassword) < 8 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填；密码至少 8 位")
	}
	if _, err := l.svcCtx.Models.User.FindOne(l.ctx, tid, in.Uid); err != nil {
		return nil, errUserNotFound
	}
	passwordHash, err := crypto.HashPassword(in.NewPassword)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if err := l.svcCtx.Models.User.UpdatePassword(l.ctx, tid, in.Uid, passwordHash, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if n, kerr := l.svcCtx.Sessions.RevokeAll(l.ctx, in.Uid); kerr != nil {
		l.Errorf("重置密码踢会话失败 uid=%d: %v", in.Uid, kerr)
	} else {
		l.Infof("重置密码 uid=%d，注销会话 %d 个", in.Uid, n)
	}
	auditLogin(l.ctx, l.svcCtx.Conn, in.Uid, tid, "auth.password_reset")
	return &pb.ResetPasswordResp{}, nil
}
