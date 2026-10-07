package logic

import (
	"context"

	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type UpdateRoleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateRoleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateRoleLogic {
	return &UpdateRoleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateRole 编辑角色 + 重设菜单权限树（全删全插）。
// 联动：该角色全部用户 auth_cache 失效重建（降权/扩权即时生效，02 §9.5）。
func (l *UpdateRoleLogic) UpdateRole(in *pb.UpdateRoleReq) (*pb.UpdateRoleResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.RoleId <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("role_id 必填")
	}
	cur, err := l.svcCtx.Models.Role.FindOne(l.ctx, tid, in.RoleId)
	if err != nil {
		return nil, errRoleNotFound
	}
	name, dataScope, remark := cur.Name, cur.DataScope.String, cur.Remark.String
	if in.Name != "" {
		name = in.Name
	}
	if in.DataScope != "" {
		dataScope = in.DataScope
	}
	if in.Remark != "" {
		remark = in.Remark
	}
	if err := l.svcCtx.Models.Role.Update(l.ctx, tid, in.RoleId, name, dataScope, remark, opUID(l.ctx)); err != nil {
		return nil, errcode.Internal.WithCause(err)
	}

	menuChanged := in.MenuIds != nil
	if menuChanged {
		if err := l.svcCtx.Models.RoleMenu.Replace(l.ctx, in.RoleId, in.MenuIds, tid, opUID(l.ctx)); err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
	}
	if menuChanged || in.DataScope != "" {
		uids, err := l.svcCtx.Models.UserRole.FindUserIdsByRole(l.ctx, tid, in.RoleId)
		if err != nil {
			return nil, errcode.Internal.WithCause(err)
		}
		for _, uid := range uids {
			_ = l.svcCtx.Sessions.DeleteAuthCache(l.ctx, uid)
			_ = ensureAuthCache(l.ctx, l.svcCtx, tid, uid)
		}
	}
	return &pb.UpdateRoleResp{}, nil
}
