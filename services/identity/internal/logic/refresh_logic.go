package logic

import (
	"context"
	"errors"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type RefreshLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Refresh 轮换式刷新（02 §9.5）：
// GETDEL 原子消费 rtid → 旧 rtid 落墓碑 → 换发新 rtid + 重签 access；
// 重用（已消费 rtid 再次出现）→ 该用户全端注销 + 告警审计。
func (l *RefreshLogic) Refresh(in *pb.RefreshReq) (*pb.RefreshResp, error) {
	if in.RefreshToken == "" {
		return nil, errcode.ErrBadRequest.WithMsg("refresh_token 必填")
	}

	newRTID, sid, err := l.svcCtx.Sessions.RotateRefresh(l.ctx, in.RefreshToken)
	if errors.Is(err, sessionx.ErrRefreshInvalid) {
		return nil, errcode.ErrSessionExpired
	}
	if errors.Is(err, sessionx.ErrRefreshReuse) {
		// 重用告警（RotateRefresh 内已完成全端注销）：审计 + ERROR 留痕
		l.Errorf("[安全告警] refresh token 重用 rtid=%s…已全端注销", safePrefix(in.RefreshToken, 8))
		return nil, errcode.ErrSessionRevoked
	}
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	sess, err := l.svcCtx.Sessions.Get(l.ctx, sid)
	if err != nil {
		return nil, errcode.ErrSessionRevoked
	}
	// client 漂移校验：refresh 与登录端必须一致
	if in.Client != "" && sess.Client != in.Client {
		return nil, errcode.ErrTokenInvalid
	}

	// 重签 access（同会话、level=1；step-up 状态在 Redis stepup:{sid}，不影响续签）
	claims := jwtauthClaims(l.svcCtx, sess.UID, sid, sess.Client)
	access, err := l.svcCtx.Signer.Sign(claims, accessTTL(l.svcCtx))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	return &pb.RefreshResp{
		Tokens: tokenPair(access, newRTID, l.svcCtx.AccessTTL, l.svcCtx.RefreshTTL),
	}, nil
}
