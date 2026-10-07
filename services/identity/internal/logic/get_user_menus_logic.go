package logic

import (
	"context"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
	"micro-server/services/identity/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type GetUserMenusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserMenusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserMenusLogic {
	return &GetUserMenusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetUserMenus 用户可见菜单（/auth/menus 数据源）：
// 用户全部角色 → role_menu 菜单并集 → type=1/2 且 status=1 的目录与页面；
// perms 返回按钮/接口级权限码（前端按钮显隐 + BFF perm resolver 对账）。
func (l *GetUserMenusLogic) GetUserMenus(in *pb.GetUserMenusReq) (*pb.GetUserMenusResp, error) {
	uid := in.Uid
	if uid == 0 {
		uid = opUID(l.ctx)
	}
	if uid <= 0 {
		return nil, errcode.ErrBadRequest.WithMsg("uid 必填")
	}
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	roleIds, err := l.svcCtx.Models.UserRole.FindRoleIdsByUser(l.ctx, tid, uid)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	menuIds, err := l.svcCtx.Models.RoleMenu.FindMenuIdsByRoles(l.ctx, tid, roleIds)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	menus := make([]*pb.MenuItem, 0, len(menuIds))
	perms := make([]string, 0)
	for _, mid := range menuIds {
		m, err := l.svcCtx.Models.Menu.FindOne(l.ctx, tid, mid)
		if err != nil {
			if err == model.ErrNotFound {
				continue
			}
			return nil, errcode.Internal.WithCause(err)
		}
		if m.Status != 1 {
			continue
		}
		item := &pb.MenuItem{
			MenuId:   m.MenuId,
			ParentId: m.ParentId,
			Name:     m.Name,
			Type:     int32(m.Type),
			PermCode: m.PermCode.String,
			Path:     m.Path.String,
			Icon:     m.Icon.String,
			Sort:     int32(m.Sort),
			Status:   int32(m.Status),
		}
		if m.Type <= 2 { // 目录/页面进菜单树
			menus = append(menus, item)
		}
		if m.PermCode.Valid && m.PermCode.String != "" { // 按钮/接口权限码进 perms
			perms = append(perms, m.PermCode.String)
		}
	}
	return &pb.GetUserMenusResp{Menus: menus, Perms: perms}, nil
}
