// bind_phone_by_code_logic.go · need_bind_phone 流程收尾（S9-01）：
// code 复用登录凭证定位影子账号 → 绑定手机号（加密+hash+查重）→ 建会话签发 token。

package logic

import (
	"context"
	"os"
	"strings"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type BindPhoneByCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindPhoneByCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPhoneByCodeLogic {
	return &BindPhoneByCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindPhoneByCode 未绑定期不签发 token，端侧绑定动作以登录 code 为凭证定位账号；
// 绑定成功即建会话（与 LoginByWechat 已绑分支同款签发链）。
func (l *BindPhoneByCodeLogic) BindPhoneByCode(in *pb.BindPhoneByCodeReq) (*pb.LoginByWechatResp, error) {
	if in.Code == "" || in.Phone == "" || in.Client == "" {
		return nil, errcode.ErrBadRequest.WithMsg("code/phone/client 必填")
	}

	// ① code → openid（与 LoginByWechat 同一套模拟/真实分支）
	client := in.Client
	if strings.EqualFold(os.Getenv("MOCKWX"), "1") {
		// dev 微信模拟：openid 由 code 确定性派生
	} else if l.svcCtx.Config.Wechat.AppID == "" || l.svcCtx.Config.Wechat.AppSecret == "" {
		return nil, errWechatNotConf
	}
	var openid string
	if strings.EqualFold(os.Getenv("MOCKWX"), "1") {
		openid = "mock-openid-" + in.Code
	} else {
		wxs, err := code2session(l.ctx, l.svcCtx.Config.Wechat.AppID, l.svcCtx.Config.Wechat.AppSecret, in.Code)
		if err != nil {
			return nil, errcode.ErrInvalidCredential.WithMsg("微信登录失败").WithCause(err)
		}
		openid = wxs.OpenID
	}

	// ② 绑定定位（必须已存在影子账号；未注册先登录）
	binding, err := l.svcCtx.Models.SsoBinding.FindByOpenid(l.ctx, "WECHAT_MA", openid)
	if err != nil {
		if err == model.ErrNotFound {
			return nil, errcode.ErrInvalidCredential.WithMsg("请先完成微信登录")
		}
		return nil, errcode.Internal.WithCause(err)
	}
	user, err := l.svcCtx.Models.User.FindOneByUID(l.ctx, binding.UserId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if !checkClientType(user.Types, client) {
		return nil, errClientNotAllowed
	}

	// ③ 手机号查重 + 加密落库（types 追加当前端）
	hash := hmacHex(l.svcCtx.HashKey, in.Phone)
	dup, err := l.svcCtx.Models.User.FindOneByMobileHash(l.ctx, hash)
	if err != nil && err != model.ErrNotFound {
		return nil, errcode.Internal.WithCause(err)
	}
	if dup != nil && dup.UserId != user.UserId {
		return nil, errMobileUsed
	}
	types := mergeClientType(user.Types, client)
	mobileCipher, err := l.svcCtx.Encryptor.Encrypt([]byte(in.Phone))
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	if err := l.svcCtx.Models.User.BindMobile(l.ctx, user.TenantId, user.UserId, types, mobileCipher, hash, ctxkit.UID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	// ④ 会话 + auth_cache + 审计（签发与 LoginByWechat 已绑分支一致）
	if err := ensureAuthCache(l.ctx, l.svcCtx, user.TenantId, user.UserId); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	access, refresh, accessExp, refreshExp, _, err := issueTokens(l.ctx, l.svcCtx, user.UserId, client)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	auditLogin(l.ctx, l.svcCtx.Conn, user.UserId, user.TenantId, "auth.bind_phone")

	l.Infof("绑定手机号并签发会话 uid=%d client=%s", user.UserId, client)
	return &pb.LoginByWechatResp{
		Tokens:        tokenPair(access, refresh, accessExp, refreshExp),
		Uid:           user.UserId,
		NeedBindPhone: false,
	}, nil
}
