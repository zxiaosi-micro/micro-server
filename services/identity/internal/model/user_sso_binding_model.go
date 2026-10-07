// user_sso_binding 表 custom 覆写（ADR-08）：微信登录按 (provider, openid) 全局 UK 定位
//（登录路径，租户上下文尚不可得，Exempt 说明见 registry.go）；管理面查询带租户过滤。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserSsoBindingModel = (*customUserSsoBindingModel)(nil)

type (
	// UserSsoBindingModel SSO 绑定模型。
	UserSsoBindingModel interface {
		// Insert 新建绑定（生成方法全列 INSERT，含 tenant_id）。
		Insert(ctx context.Context, data *UserSsoBinding) (sql.Result, error)
		// FindByOpenid 登录路径定位（provider+openid 全局 UK）。
		FindByOpenid(ctx context.Context, provider, openid string) (*UserSsoBinding, error)
		// FindByUser 用户全部绑定（管理面，租户过滤）。
		FindByUser(ctx context.Context, tenantId, userId int64) ([]*UserSsoBinding, error)
	}

	customUserSsoBindingModel struct {
		*defaultUserSsoBindingModel
	}
)

// NewUserSsoBindingModel returns a model for the database table.
func NewUserSsoBindingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserSsoBindingModel {
	return &customUserSsoBindingModel{
		defaultUserSsoBindingModel: newUserSsoBindingModel(conn, c, opts...),
	}
}

func (m *customUserSsoBindingModel) FindByOpenid(ctx context.Context, provider, openid string) (*UserSsoBinding, error) {
	var resp UserSsoBinding
	query := fmt.Sprintf("select %s from %s where `provider` = ? and `openid` = ? and `deleted_at` is null limit 1", userSsoBindingRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, provider, openid)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customUserSsoBindingModel) FindByUser(ctx context.Context, tenantId, userId int64) ([]*UserSsoBinding, error) {
	query := fmt.Sprintf("select %s from %s where `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", userSsoBindingRows, m.table)
	var list []*UserSsoBinding
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, userId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}
