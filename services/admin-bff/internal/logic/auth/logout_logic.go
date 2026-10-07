package auth

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Logout 登出：sid 由 BearerProbe 注入 ctx（免鉴权组，持有效 token 即可登出）。
func (l *LogoutLogic) Logout() (*types.SimpleResp, error) {
	sid := ctxkit.SID(l.ctx)
	uid := ctxkit.UID(l.ctx)
	if sid == "" || uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	if _, err := l.svcCtx.Identity.Logout(l.ctx, &pb.LogoutReq{Sid: sid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: strconv.FormatInt(uid, 10)}, nil
}
