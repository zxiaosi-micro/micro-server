// greeting 表 custom 覆写（S3-06 模板示例 · ADR-08 口径）。
//
// 新服务照抄本文件的写法（identity 各 model 同构）：
//   - 生成方法（_gen.go）只保留 Insert/主键 FindOne/Update/Delete；
//   - 租户过滤查询覆写在 custom：显式 `tenant_id = ? AND deleted_at IS NULL`，
//     且走 QueryRowNoCacheCtx（NoCache）或带 tenant 前缀的手工缓存键；
//   - INSERT 含 tenant_id（Logic 层经 tenantx.MustTenantFromCtx 填入）。
//
// ⚠ goctl 重生成只写 _gen.go——本文件的覆写不会被覆盖（custom 文件覆写纪律，02 §6.2 ①）。

package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/tenantx"
)

var _ GreetingModel = (*customGreetingModel)(nil)

func init() {
	// tenantaudit 对账登记（02 §9.4）：每张租户表在 model 包 init 声明归属。
	tenantx.Scoped("greeting")
}

type (
	// GreetingModel 模板示例模型：生成方法之外，追加租户过滤查询。
	GreetingModel interface {
		greetingModel
		// FindOneTenant 租户过滤单查（模板演示：同结构服务照抄）。
		FindOneTenant(ctx context.Context, tenantId, id int64) (*Greeting, error)
		// FindFirstByTenant 租户内第一条（/greeting 演示路由的数据源）。
		FindFirstByTenant(ctx context.Context, tenantId int64) (*Greeting, error)
	}

	customGreetingModel struct {
		*defaultGreetingModel
	}
)

// NewGreetingModel returns a model for the database table.
func NewGreetingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) GreetingModel {
	return &customGreetingModel{
		defaultGreetingModel: newGreetingModel(conn, c, opts...),
	}
}

// FindOneTenant 租户过滤单查：显式列名（禁 SELECT *）+ NoCacheCtx（②款：租户敏感查询不走共享行缓存）。
func (m *customGreetingModel) FindOneTenant(ctx context.Context, tenantId, id int64) (*Greeting, error) {
	var resp Greeting
	query := fmt.Sprintf("select %s from %s where `id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", greetingRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, id, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// FindFirstByTenant 租户内第一条问候。
func (m *customGreetingModel) FindFirstByTenant(ctx context.Context, tenantId int64) (*Greeting, error) {
	var resp Greeting
	query := fmt.Sprintf("select %s from %s where `tenant_id` = ? and `deleted_at` is null order by `id` limit 1", greetingRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
