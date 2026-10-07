package user

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteUserLogic {
	return &DeleteUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// DeleteUser 软删（identity 联动解绑角色/踢会话/失效 auth_cache）。
func (l *DeleteUserLogic) DeleteUser(req *types.IDPath) (*types.SimpleResp, error) {
	uid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
	}
	if _, err := l.svcCtx.Identity.DeleteUser(l.ctx, &pb.DeleteUserReq{Uid: uid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.ID}, nil
}
