package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type LoginByWechatLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginByWechatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginByWechatLogic {
	return &LoginByWechatLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// wxSession 微信 code2session 响应（S9 客户端小程序主链路；S3 落通骨架与绑定语义）。
type wxSession struct {
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// LoginByWechat 微信登录：code2session → SSO 绑定定位 →
// 未绑手机 → need_bind_phone=true（前端走绑定流程，不签发 token）；
// 已绑 → 建会话签发。dev 未配 Wechat.* 返回 1101013 明确报错。
func (l *LoginByWechatLogic) LoginByWechat(in *pb.LoginByWechatReq) (*pb.LoginByWechatResp, error) {
	if in.Code == "" || in.Client == "" {
		return nil, errcode.ErrBadRequest.WithMsg("code/client 必填")
	}
	appID := l.svcCtx.Config.Wechat.AppID
	appSecret := l.svcCtx.Config.Wechat.AppSecret
	if appID == "" || appSecret == "" {
		return nil, errWechatNotConf
	}

	// ① code2session（jscode2interface；GET https://api.weixin.qq.com/sns/jscode2session）
	wxs, err := code2session(l.ctx, appID, appSecret, in.Code)
	if err != nil {
		return nil, errcode.ErrInvalidCredential.WithMsg("微信登录失败").WithCause(err)
	}

	// ② SSO 绑定定位（provider+openid 全局 UK；登录路径租户上下文尚不可得）
	binding, err := l.svcCtx.Models.SsoBinding.FindByOpenid(l.ctx, "WECHAT_MA", wxs.OpenID)
	if err != nil && err != model.ErrNotFound {
		return nil, errcode.Internal.WithCause(err)
	}
	if binding == nil {
		// 未绑定：创建影子账号（无手机无密码，types 含该端）+ 绑定行；need_bind_phone 由前端引导
		uid, err := l.createShadowUser(wxs)
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		return &pb.LoginByWechatResp{Uid: uid, NeedBindPhone: true}, nil
	}

	// ③ 已绑定：校验端类型 + 状态
	user, err := l.svcCtx.Models.User.FindOneByUID(l.ctx, binding.UserId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if !checkClientType(user.Types, in.Client) {
		return nil, errClientNotAllowed
	}
	if user.Status == 2 {
		if _, kerr := l.svcCtx.Sessions.RevokeAll(l.ctx, user.UserId); kerr != nil {
			l.Errorf("禁用用户踢会话失败 uid=%d: %v", user.UserId, kerr)
		}
		return nil, errcode.ErrAccountDisabled
	}

	// ④ 会话 + auth_cache + 审计
	if err := ensureAuthCache(l.ctx, l.svcCtx, user.TenantId, user.UserId); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	access, refresh, accessExp, refreshExp, _, err := issueTokens(l.ctx, l.svcCtx, user.UserId, in.Client)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	auditLogin(l.ctx, l.svcCtx.Conn, user.UserId, user.TenantId, "auth.login_wechat")

	return &pb.LoginByWechatResp{
		Tokens: tokenPair(access, refresh, accessExp, refreshExp),
		Uid:    user.UserId,
	}, nil
}

// createShadowUser 影子账号：openid 首登建 user（nickname=微信用户，types=当前端）+ sso 绑定。
func (l *LoginByWechatLogic) createShadowUser(wxs *wxSession) (int64, error) {
	client := ctxkit.Client(l.ctx)
	if client == "" {
		client = ctxkit.ClientClientMini
	}
	tenantID, err := defaultTenantID(l.svcCtx)
	if err != nil {
		return 0, err
	}
	types, err := json.Marshal([]string{client})
	if err != nil {
		return 0, err
	}
	uid := l.svcCtx.Snowflake.MustNextID()
	now := time.Now()
	_, err = l.svcCtx.Models.User.Insert(l.ctx, &model.User{
		UserId:    uid,
		Nickname:  "微信用户",
		Types:     string(types),
		TenantId:  tenantID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return 0, err
	}
	_, err = l.svcCtx.Models.SsoBinding.Insert(l.ctx, &model.UserSsoBinding{
		BindingId: l.svcCtx.Snowflake.MustNextID(),
		UserId:    uid,
		Provider:  "WECHAT_MA",
		Openid:    wxs.OpenID,
		Unionid:   toNullString(wxs.UnionID),
		TenantId:  tenantID,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return 0, err
	}
	l.Infof("微信首登建影子账号 uid=%d openid=%s…", uid, safePrefix(wxs.OpenID, 8))
	return uid, nil
}

func code2session(ctx context.Context, appID, appSecret, code string) (*wxSession, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		appID, appSecret, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out wxSession
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.ErrCode != 0 {
		return nil, errors.New("code2session errcode=" + fmt.Sprint(out.ErrCode) + " " + out.ErrMsg)
	}
	if out.OpenID == "" {
		return nil, errors.New("code2session 空 openid")
	}
	return &out, nil
}
