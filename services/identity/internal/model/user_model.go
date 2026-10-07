// user 表 custom 覆写（ADR-08）：
//   - 查询显式带 `tenant_id = ? AND deleted_at IS NULL`（登录按 mobile_hash 全局定位为唯一例外，
//     见 registry.go 登记说明）；
//   - INSERT 走生成方法（全列含 tenant_id，Logic 层经 tenantx 填入）；
//   - 更新走显式列方法，禁 UPDATE *；
//   - 租户敏感查询走 QueryRowNoCacheCtx / QueryRowsNoCacheCtx（②款）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserModel = (*customUserModel)(nil)

type (
	// UserModel 管理面用户模型（生成方法中仅暴露 Insert；查询/更新一律走本接口的租户过滤版本）。
	UserModel interface {
		// Insert 新建用户（生成方法：全列 INSERT，data.TenantId 必填，Logic 层负责）。
		Insert(ctx context.Context, data *User) (sql.Result, error)
		// FindOneByMobileHash 登录路径全局定位（mobile_hash 全局 UK；此时租户上下文尚不可得）。
		FindOneByMobileHash(ctx context.Context, mobileHash string) (*User, error)
		// FindTenantByUID 取用户归属租户（auth_cache 重建等登录后链路，全局查询）。
		FindTenantByUID(ctx context.Context, userId int64) (int64, error)
		// FindOneByUID 全局按 ID 取（step-up 等：会话内的 uid 已可信，密码校验需全行）。
		FindOneByUID(ctx context.Context, userId int64) (*User, error)
		// FindOne 管理面按 ID 取用户（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, userId int64) (*User, error)
		// FindPage 用户列表（nickname 模糊 + status 过滤，分页；total 同条件计数）。
		FindPage(ctx context.Context, tenantId int64, nickname string, status int64, page, size int64) ([]*User, int64, error)
		// UpdateProfile 编辑用户基本信息（mobile/email 传 NullString{Valid:false} 表示不变更）。
		UpdateProfile(ctx context.Context, tenantId, userId int64, nickname string, orgId, partyId sql.NullInt64,
			types string, mobile, mobileHash, email, emailHash sql.NullString, updatedBy int64) error
		// UpdateParty 参与方绑定（S4 BindParty）。
		UpdateParty(ctx context.Context, tenantId, userId, partyId, updatedBy int64) error
		// CountByOrg 挂靠指定组织的用户数（组织删除前校验）。
		CountByOrg(ctx context.Context, tenantId, orgId int64) (int64, error)
		// CountByTenant 租户用户数（租户删除前校验）。
		CountByTenant(ctx context.Context, tenantId int64) (int64, error)
		// UpdateStatus 状态变更（1 正常 / 2 禁用 / 3 锁定）。
		UpdateStatus(ctx context.Context, tenantId, userId, status, updatedBy int64) error
		// UpdateLockedUntil 锁定到期镜像（权威状态在 sessionx locked:{ident}，此列供审计展示）。
		UpdateLockedUntil(ctx context.Context, tenantId, userId int64, lockedUntil sql.NullTime, updatedBy int64) error
		// UpdatePassword 重置密码（argon2id PHC 串）。
		UpdatePassword(ctx context.Context, tenantId, userId int64, passwordHash string, updatedBy int64) error
		// SoftDelete 软删（置 deleted_at；绑定的角色/会话由 Logic 层联动清理）。
		SoftDelete(ctx context.Context, tenantId, userId, updatedBy int64) error
	}

	customUserModel struct {
		*defaultUserModel
	}
)

// NewUserModel returns a model for the database table.
func NewUserModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserModel {
	return &customUserModel{
		defaultUserModel: newUserModel(conn, c, opts...),
	}
}

func (m *customUserModel) FindOneByMobileHash(ctx context.Context, mobileHash string) (*User, error) {
	var resp User
	query := fmt.Sprintf("select %s from %s where `mobile_hash` = ? and `deleted_at` is null limit 1", userRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, mobileHash)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customUserModel) FindOne(ctx context.Context, tenantId, userId int64) (*User, error) {
	var resp User
	query := fmt.Sprintf("select %s from %s where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		userRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, userId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customUserModel) FindPage(ctx context.Context, tenantId int64, nickname string, status int64, page, size int64) ([]*User, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if nickname != "" {
		where += " and `nickname` like ?"
		args = append(args, "%"+nickname+"%")
	}
	if status > 0 {
		where += " and `status` = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `user_id` desc limit ? offset ?",
		userRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*User
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customUserModel) UpdateProfile(ctx context.Context, tenantId, userId int64, nickname string, orgId, partyId sql.NullInt64,
	types string, mobile, mobileHash, email, emailHash sql.NullString, updatedBy int64) error {
	query := fmt.Sprintf(`update %s set `+"`nickname`"+` = ?, `+"`org_id`"+` = ?, `+"`party_id`"+` = ?, `+"`types`"+` = ?,
		`+"`mobile`"+` = ?, `+"`mobile_hash`"+` = ?, `+"`email`"+` = ?, `+"`email_hash`"+` = ?, `+"`updated_by`"+` = ?, `+"`updated_at`"+` = ?
		where `+"`user_id`"+` = ? and `+"`tenant_id`"+` = ? and `+"`deleted_at`"+` is null`, m.table)
	_, err := m.ExecNoCacheCtx(ctx, query,
		nickname, orgId, partyId, types, mobile, mobileHash, email, emailHash, sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0},
		time.Now(), userId, tenantId)
	return err
}

func (m *customUserModel) UpdateStatus(ctx context.Context, tenantId, userId, status, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `status` = ?, `updated_by` = ?, `updated_at` = ? where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, status,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), userId, tenantId)
	return err
}

func (m *customUserModel) UpdateLockedUntil(ctx context.Context, tenantId, userId int64, lockedUntil sql.NullTime, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `locked_until` = ?, `status` = ?, `updated_at` = ? where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	status := int64(1)
	if lockedUntil.Valid && lockedUntil.Time.After(time.Now()) {
		status = 3
	}
	_, err := m.ExecNoCacheCtx(ctx, query, lockedUntil, status, time.Now(), userId, tenantId)
	return err
}

func (m *customUserModel) UpdatePassword(ctx context.Context, tenantId, userId int64, passwordHash string, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `password_hash` = ?, `updated_by` = ?, `updated_at` = ? where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, passwordHash,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), userId, tenantId)
	return err
}

func (m *customUserModel) SoftDelete(ctx context.Context, tenantId, userId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `status` = 2 where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, userId, tenantId)
	return err
}

func (m *customUserModel) FindTenantByUID(ctx context.Context, userId int64) (int64, error) {
	var tid int64
	query := fmt.Sprintf("select `tenant_id` from %s where `user_id` = ? and `deleted_at` is null limit 1", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &tid, query, userId); err != nil {
		return 0, err
	}
	return tid, nil
}

func (m *customUserModel) FindOneByUID(ctx context.Context, userId int64) (*User, error) {
	var resp User
	query := fmt.Sprintf("select %s from %s where `user_id` = ? and `deleted_at` is null limit 1", userRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, userId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customUserModel) UpdateParty(ctx context.Context, tenantId, userId, partyId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `party_id` = ?, `updated_by` = ?, `updated_at` = ? where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, sql.NullInt64{Int64: partyId, Valid: partyId > 0},
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), userId, tenantId)
	return err
}

func (m *customUserModel) CountByOrg(ctx context.Context, tenantId, orgId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `org_id` = ? and `deleted_at` is null", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId, orgId); err != nil {
		return 0, err
	}
	return n, nil
}

func (m *customUserModel) CountByTenant(ctx context.Context, tenantId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `deleted_at` is null", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId); err != nil {
		return 0, err
	}
	return n, nil
}
