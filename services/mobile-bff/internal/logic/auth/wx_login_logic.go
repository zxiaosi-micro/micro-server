// wx_login_logic.go · 微信登录（客户端小程序；client 固定 CLIENT_MINI；
// dev MOCKWX=1 由 identity 模拟 code2session——04 §3 F2）。

package auth

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// miniClient 客户端小程序固定登录端。
const miniClient = "CLIENT_MINI"

type WxLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWxLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WxLoginLogic {
	return &WxLoginLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WxLogin 微信静默登录 → 自动注册影子账号 → need_bind_phone 引导绑定。
func (l *WxLoginLogic) WxLogin(req *types.WxLoginReq) (resp *types.WxLoginResp, err error) {
	out, err := l.svcCtx.Identity.LoginByWechat(l.ctx, &ipb.LoginByWechatReq{
		Code:   req.Code,
		Client: miniClient,
	})
	if err != nil {
		return nil, err
	}
	resp = &types.WxLoginResp{NeedBindPhone: out.NeedBindPhone, UID: i64s(out.Uid)}
	if out.Tokens != nil {
		resp.Tokens = tokenPair(out.Tokens)
	}
	return resp, nil
}
