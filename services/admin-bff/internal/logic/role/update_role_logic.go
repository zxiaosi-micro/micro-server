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

type UpdateRoleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRoleLogic {
	return &UpdateRoleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateRole 编辑角色 + 重设权限树（identity 联动失效相关用户 auth_cache）。
func (l *UpdateRoleLogic) UpdateRole(req *types.RoleUpdateReq) (*types.SimpleResp, error) {
	rid, err := strconv.ParseInt(req.RoleID, 10, 64)
	if err != nil {
		return nil, errcode.ErrBadRequest.WithMsg("role_id 非法")
	}
	if _, err := l.svcCtx.Identity.UpdateRole(l.ctx, &pb.UpdateRoleReq{
		RoleId: rid, Name: req.Name, DataScope: req.DataScope,
		Remark: req.Remark, MenuIds: parseIDs(req.MenuIDs),
	}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{UID: req.RoleID}, nil
}
