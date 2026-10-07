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

type UpdateUserStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserStatusLogic {
	return &UpdateUserStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateUserStatus 禁用/恢复（禁用即时踢全部会话，由 identity 执行）。
func (l *UpdateUserStatusLogic) UpdateUserStatus(req *types.UserStatusReq) (*types.SimpleResp, error) {
	uid, err := strconv.ParseInt(req.UID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("uid 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateUserStatus(l.ctx, &pb.UpdateUserStatusReq{
		Uid: uid, Status: int32(req.Status),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.UID}, nil
}
