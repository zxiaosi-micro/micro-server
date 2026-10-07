// role_menu 表（联合主键 role_id+menu_id，goctl model 不支持联合主键——手写，ADR-08 同口径）。
// 菜单权限树保存 = 全删全插（事务内），查询显式 tenant_id。

package model

import (
	"context"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type (
	// RoleMenuModel 角色-菜单绑定模型。
	RoleMenuModel interface {
		// Replace 重设角色菜单（事务内全删全插）。
		Replace(ctx context.Context, roleId int64, menuIds []int64, tenantId, createdBy int64) error
		// DeleteByRole 角色软删时联动清理。
		DeleteByRole(ctx context.Context, tenantId, roleId int64) error
		// DeleteByMenu 菜单软删时联动清理（全部租户内该菜单）。
		DeleteByMenu(ctx context.Context, tenantId, menuId int64) error
		// FindMenuIdsByRole 角色菜单 ID 列表（角色编辑回显 + /auth/menus 组装）。
		FindMenuIdsByRole(ctx context.Context, tenantId, roleId int64) ([]int64, error)
		// FindMenuIdsByRoles 多角色并集（/auth/menus：用户全部角色的菜单聚合）。
		FindMenuIdsByRoles(ctx context.Context, tenantId int64, roleIds []int64) ([]int64, error)
	}

	roleMenuModel struct {
		conn sqlx.SqlConn
	}
)

// NewRoleMenuModel 构造。
func NewRoleMenuModel(conn sqlx.SqlConn) RoleMenuModel {
	return &roleMenuModel{conn: conn}
}

func (m *roleMenuModel) Replace(ctx context.Context, roleId int64, menuIds []int64, tenantId, createdBy int64) error {
	if err := m.DeleteByRole(ctx, tenantId, roleId); err != nil {
		return err
	}
	if len(menuIds) == 0 {
		return nil
	}
	query := "insert ignore into `role_menu` (`role_id`,`menu_id`,`tenant_id`,`created_at`,`created_by`) values (?, ?, ?, ?, ?)"
	now := time.Now()
	for _, mid := range menuIds {
		if _, err := m.conn.ExecCtx(ctx, query, roleId, mid, tenantId, now, createdBy); err != nil {
			return err
		}
	}
	return nil
}

func (m *roleMenuModel) DeleteByRole(ctx context.Context, tenantId, roleId int64) error {
	_, err := m.conn.ExecCtx(ctx, "delete from `role_menu` where `role_id` = ? and `tenant_id` = ?", roleId, tenantId)
	return err
}

func (m *roleMenuModel) DeleteByMenu(ctx context.Context, tenantId, menuId int64) error {
	_, err := m.conn.ExecCtx(ctx, "delete from `role_menu` where `menu_id` = ? and `tenant_id` = ?", menuId, tenantId)
	return err
}

func (m *roleMenuModel) FindMenuIdsByRole(ctx context.Context, tenantId, roleId int64) ([]int64, error) {
	var ids []int64
	err := m.conn.QueryRowsCtx(ctx, &ids,
		"select `menu_id` from `role_menu` where `role_id` = ? and `tenant_id` = ?", roleId, tenantId)
	return ids, err
}

func (m *roleMenuModel) FindMenuIdsByRoles(ctx context.Context, tenantId int64, roleIds []int64) ([]int64, error) {
	if len(roleIds) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(roleIds))
	args := make([]any, 0, len(roleIds)+1)
	args = append(args, tenantId)
	for i, rid := range roleIds {
		placeholders[i] = "?"
		args = append(args, rid)
	}
	query := "select distinct `menu_id` from `role_menu` where `tenant_id` = ? and `role_id` in (" +
		strings.Join(placeholders, ",") + ")"
	var ids []int64
	if err := m.conn.QueryRowsCtx(ctx, &ids, query, args...); err != nil {
		return nil, err
	}
	return ids, nil
}
