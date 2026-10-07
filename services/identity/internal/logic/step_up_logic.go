package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type StepUpLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewStepUpLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StepUpLogic {
	return &StepUpLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// StepUp 二次认证（02 §9.5）：校验密码 → RaiseLevel(sid,2)（Redis TTL 5min）→
// 重签 level=2 的 access token（BFF SensitivePrefixes 路由要求 claims.Level≥2）。
func (l *StepUpLogic) StepUp(in *pb.StepUpReq) (*pb.StepUpResp, error) {
	if in.Sid == "" || in.Password == "" {
		return nil, errcode.ErrBadRequest.WithMsg("sid/password 必填")
	}

	sess, err := l.svcCtx.Sessions.Get(l.ctx, in.Sid)
	if err != nil {
		if err == sessionx.ErrSessionNotFound {
			return nil, errcode.ErrSessionRevoked
		}
		return nil, errcode.Internal.WithCause(err)
	}

	// 凭证校验（uid 定位密码；失败不锁定——step-up 不是登录入口，只拒绝）
	user, err := l.svcCtx.Models.User.FindOneByUID(l.ctx, sess.UID)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if !user.PasswordHash.Valid {
		return nil, errcode.ErrInvalidCredential.WithMsg("当前账号无密码,不可二次验证")
	}
	ok, err := crypto.VerifyPassword(in.Password, user.PasswordHash.String)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if !ok {
		return nil, errcode.ErrInvalidCredential
	}

	if err := l.svcCtx.Sessions.RaiseLevel(l.ctx, in.Sid, 2); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	claims := jwtauthClaims(l.svcCtx, sess.UID, in.Sid, sess.Client)
	claims.Level = 2
	access, err := l.svcCtx.Signer.Sign(claims, accessTTL(l.svcCtx))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	auditLogin(l.ctx, l.svcCtx.Conn, sess.UID, user.TenantId, "auth.stepup")
	return &pb.StepUpResp{
		Level:           2,
		AccessToken:     access,
		AccessExpiresIn: l.svcCtx.AccessTTL,
	}, nil
}
