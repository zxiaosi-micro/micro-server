// bind_phone_logic.go · 绑定手机号（need_bind_phone 流程收尾；加密/查重在 identity）。

package customer

import (
	"context"

	ipb "micro-server/services/identity/pb"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type BindPhoneLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBindPhoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPhoneLogic {
	return &BindPhoneLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *BindPhoneLogic) BindPhone(req *types.BindPhoneReq) (resp *types.SimpleResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	if _, err := l.svcCtx.Identity.BindPhone(l.ctx, &ipb.BindPhoneReq{
		Uid:    uid,
		Phone:  req.Phone,
		Client: "CLIENT_MINI",
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
