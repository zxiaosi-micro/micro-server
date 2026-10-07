// user_role 表（联合主键 user_id+role_id，goctl model 不支持联合主键——手写，ADR-08 同口径：
// 查询显式 tenant_id + 软删过滤；绑定表不做行缓存，走 NoCacheCtx）。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const userRoleColumns = "`user_id`,`role_id`,`tenant_id`,`created_at`,`created_by`,`updated_at`,`updated_by`,`deleted_at`"

type (
	// UserRoleModel 用户-角色绑定模型。
	UserRoleModel interface {
		// BatchInsert 批量绑定（INSERT IGNORE 幂等：已绑定跳过）。
		BatchInsert(ctx context.Context, userId int64, roleIds []int64, tenantId, createdBy int64) error
		// DeleteByUser 解绑用户全部角色（编辑角色保存/软删用户时）。
		DeleteByUser(ctx context.Context, tenantId, userId int64) error
		// DeleteByRole 解绑角色全部用户（角色软删时）。
		DeleteByRole(ctx context.Context, tenantId, roleId int64) error
		// FindRoleIdsByUser 用户角色 ID 列表（auth_cache 组装源）。
		FindRoleIdsByUser(ctx context.Context, tenantId, userId int64) ([]int64, error)
		// FindUserIdsByRole 角色下的用户 ID 列表（角色变更后失效这些用户的 auth_cache）。
		FindUserIdsByRole(ctx context.Context, tenantId, roleId int64) ([]int64, error)
	}

	userRoleModel struct {
		conn sqlx.SqlConn
	}
)

// NewUserRoleModel 构造。
func NewUserRoleModel(conn sqlx.SqlConn) UserRoleModel {
	return &userRoleModel{conn: conn}
}

func (m *userRoleModel) BatchInsert(ctx context.Context, userId int64, roleIds []int64, tenantId, createdBy int64) error {
	if len(roleIds) == 0 {
		return nil
	}
	query := "insert ignore into `user_role` (`user_id`,`role_id`,`tenant_id`,`created_at`,`created_by`) values (?, ?, ?, ?, ?)"
	now := time.Now()
	for _, rid := range roleIds {
		if _, err := m.conn.ExecCtx(ctx, query, userId, rid, tenantId, now, createdBy); err != nil {
			return err
		}
	}
	return nil
}

func (m *userRoleModel) DeleteByUser(ctx context.Context, tenantId, userId int64) error {
	_, err := m.conn.ExecCtx(ctx, "delete from `user_role` where `user_id` = ? and `tenant_id` = ?", userId, tenantId)
	return err
}

func (m *userRoleModel) DeleteByRole(ctx context.Context, tenantId, roleId int64) error {
	_, err := m.conn.ExecCtx(ctx, "delete from `user_role` where `role_id` = ? and `tenant_id` = ?", roleId, tenantId)
	return err
}

func (m *userRoleModel) FindRoleIdsByUser(ctx context.Context, tenantId, userId int64) ([]int64, error) {
	var ids []int64
	err := m.conn.QueryRowsCtx(ctx, &ids,
		"select `role_id` from `user_role` where `user_id` = ? and `tenant_id` = ?", userId, tenantId)
	return ids, err
}

func (m *userRoleModel) FindUserIdsByRole(ctx context.Context, tenantId, roleId int64) ([]int64, error) {
	var ids []int64
	err := m.conn.QueryRowsCtx(ctx, &ids,
		"select `user_id` from `user_role` where `role_id` = ? and `tenant_id` = ?", roleId, tenantId)
	return ids, err
}
