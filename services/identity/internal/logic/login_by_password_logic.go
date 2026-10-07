package logic

import (
	"context"
	"errors"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type LoginByPasswordLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginByPasswordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginByPasswordLogic {
	return &LoginByPasswordLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// LoginByPassword 密码登录主链（02 §9.5）：
// 锁定校验（10407 倒计时）→ argon2id 校验（失败计数 5 次锁 30min）→
// types 端校验 → 状态校验（禁用踢全部）→ auth_cache 写入 → 互斥踢旧签发 → auth.login 审计。
func (l *LoginByPasswordLogic) LoginByPassword(in *pb.LoginByPasswordReq) (*pb.LoginResp, error) {
	if in.Mobile == "" || in.Password == "" || in.Client == "" {
		return nil, errcode.ErrBadRequest.WithMsg("mobile/password/client 必填")
	}

	mobileHash := l.hashIndex(in.Mobile)
	ident := loginIdent(mobileHash)

	// ① 锁定校验（Redis 快路径；PTTL 毫秒精度喂倒计时）
	state, err := l.svcCtx.Sessions.CheckLocked(l.ctx, ident)
	if err != nil {
		l.Errorf("CheckLocked 失败 ident=%.8s: %v", ident, err)
		return nil, errcode.ErrServiceUnavailable.WithMsg("登录服务暂不可用")
	}
	if state.Locked {
		return nil, wrapLocked(state)
	}

	// ② 定位用户（mobile_hash 全局 UK；登录路径租户上下文尚不可得，registry.go 登记说明）
	user, err := l.svcCtx.Models.User.FindOneByMobileHash(l.ctx, mobileHash)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		l.Errorf("FindOneByMobileHash 失败: %v", err)
		return nil, errcode.Internal.WithCause(err)
	}

	// ③ 凭证校验（不区分账号不存在/密码错误——10400 防遍历；失败计数走同 ident）
	if user == nil || !user.PasswordHash.Valid {
		return nil, l.recordFail(ident)
	}
	ok, err := crypto.VerifyPassword(in.Password, user.PasswordHash.String)
	if err != nil {
		l.Errorf("VerifyPassword 失败 uid=%d: %v", user.UserId, err)
		return nil, errcode.Internal.WithCause(err)
	}
	if !ok {
		return nil, l.recordFail(ident)
	}

	// ④ 端类型校验（单账号多端：user.types JSON 集合）
	if !checkClientType(user.Types, in.Client) {
		return nil, errClientNotAllowed
	}

	// ⑤ 状态校验：禁用即时踢全部会话（locked 权威在 Redis locked:{ident}）
	if user.Status == 2 {
		if _, kerr := l.svcCtx.Sessions.RevokeAll(l.ctx, user.UserId); kerr != nil {
			l.Errorf("禁用用户踢会话失败 uid=%d: %v", user.UserId, kerr)
		}
		return nil, errcode.ErrAccountDisabled
	}

	// ⑥ auth_cache 写入（角色→权限码集合；BFF 每请求 GET）
	if err := ensureAuthCache(l.ctx, l.svcCtx, user.TenantId, user.UserId); err != nil {
		l.Errorf("ensureAuthCache 失败 uid=%d: %v", user.UserId, err)
		return nil, errcode.Internal.WithCause(err)
	}

	// ⑦ 互斥踢旧（同端互斥 QQ 模式）+ 签发 token 对
	access, refresh, accessExp, refreshExp, kicked, err := issueTokens(l.ctx, l.svcCtx, user.UserId, in.Client)
	if err != nil {
		l.Errorf("issueTokens 失败 uid=%d: %v", user.UserId, err)
		return nil, errcode.Internal.WithCause(err)
	}
	for _, sid := range kicked {
		l.Infof("互斥踢旧 uid=%d sid=%s client=%s", user.UserId, sid, in.Client)
	}

	// ⑧ 锁定计数清零（登录成功）
	_ = l.svcCtx.Sessions.ClearLoginFails(l.ctx, ident)

	// ⑨ auth.login 审计（outbox；互斥踢旧附带 kick 事件）
	auditLogin(l.ctx, l.svcCtx.Conn, user.UserId, user.TenantId, "auth.login")
	if len(kicked) > 0 {
		auditLogin(l.ctx, l.svcCtx.Conn, user.UserId, user.TenantId, "auth.kick_old")
	}

	return &pb.LoginResp{
		Tokens:   tokenPair(access, refresh, accessExp, refreshExp),
		Uid:      user.UserId,
		Nickname: user.Nickname,
	}, nil
}

// recordFail 记录一次失败；达到阈值 → 锁定并返回 10407（前端倒计时）。
func (l *LoginByPasswordLogic) recordFail(ident string) error {
	res, err := l.svcCtx.Sessions.RecordLoginFail(l.ctx, ident,
		l.svcCtx.Config.Lockout.MaxFails,
		l.svcCtx.Config.Lockout.FailWindow,
		l.svcCtx.Config.Lockout.LockDur)
	if err != nil {
		// 计数失败不阻断 10400 语义（防遍历优先），留痕
		l.Errorf("登录失败计数写入失败: %v", err)
		return errcode.ErrInvalidCredential
	}
	if res.Locked {
		return errcode.ErrLoginLocked // 首次触发锁定：前端按默认 30min 倒计时
	}
	return errcode.ErrInvalidCredential
}

func tokenPair(access, refresh string, accessExp, refreshExp int64) *pb.TokenPair {
	return &pb.TokenPair{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresIn:  accessExp,
		RefreshExpiresIn: refreshExp,
	}
}
