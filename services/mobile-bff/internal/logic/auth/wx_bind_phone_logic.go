// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type WxBindPhoneLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 微信绑定手机号并签发会话（need_bind_phone 流程收尾；code 定位影子账号）
func NewWxBindPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WxBindPhoneLogic {
	return &WxBindPhoneLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WxBindPhone 绑定手机号并签发会话（identity 以 code 定位影子账号；client 固定 CLIENT_MINI）。
func (l *WxBindPhoneLogic) WxBindPhone(req *types.WxBindPhoneReq) (resp *types.WxLoginResp, err error) {
	out, err := l.svcCtx.Identity.BindPhoneByCode(l.ctx, &ipb.BindPhoneByCodeReq{
		Code:   req.Code,
		Phone:  req.Phone,
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
