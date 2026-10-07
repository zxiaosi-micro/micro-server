package role

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type DeleteRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteRoleLogic {
	return &DeleteRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteRoleLogic) DeleteRole(req *types.IDPath) (*types.SimpleResp, error) {
	rid, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("role_id 非法")
	}
	if _, err := l.svcCtx.Identity.DeleteRole(l.ctx, &pb.DeleteRoleReq{RoleId: rid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.ID}, nil
}
