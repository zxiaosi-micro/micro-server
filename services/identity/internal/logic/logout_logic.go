package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/sessionx"
)

type LogoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Logout 主动登出：写 logout 墓碑（BFF 后续请求命中 10403）。
func (l *LogoutLogic) Logout(in *pb.LogoutReq) (*pb.LogoutResp, error) {
	if in.Sid == "" {
		return nil, errcode.ErrBadRequest.WithMsg("sid 必填")
	}
	if err := l.svcCtx.Sessions.Revoke(l.ctx, in.Sid, sessionx.ReasonLogout); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	uid := opUID(l.ctx)
	if uid > 0 {
		auditLogin(l.ctx, l.svcCtx.Conn, uid, 0, "auth.logout")
	}
	return &pb.LogoutResp{}, nil
}
