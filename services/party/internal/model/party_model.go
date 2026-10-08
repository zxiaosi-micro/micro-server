// party 表 custom 覆写（ADR-08）：
//   - 查询显式带 `tenant_id = ? AND deleted_at IS NULL`；
//   - INSERT 走生成方法（全列含 tenant_id，Logic 层经 tenantx 填入）；
//   - 更新走显式列方法，禁 UPDATE *；
//   - 租户敏感查询走 QueryRowNoCacheCtx / QueryRowsNoCacheCtx（02 §6.2 ②款）。
//
// ⚠ goctl 重生成只写 _gen.go——本文件覆写不会被覆盖。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PartyModel = (*customPartyModel)(nil)

type (
	// PartyModel 参与方模型：仅暴露租户过滤版本，生成 FindOne/Update/Delete 不外泄。
	PartyModel interface {
		// Insert 新建参与方（生成方法：全列 INSERT，data.TenantId 必填，Logic 层负责）。
		Insert(ctx context.Context, data *Party) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤 + 软删过滤）。
		FindOne(ctx context.Context, tenantId, partyId int64) (*Party, error)
		// FindPage 参与方列表（keyword 模糊 name/credit_code；typ 过滤 JSON 数组包含；status 0=全部）。
		FindPage(ctx context.Context, tenantId int64, keyword, typ string, status int64, page, size int64) ([]*Party, int64, error)
		// UpdateColumns 显式列更新（02 §6.2 ①款：禁 UPDATE *）。
		UpdateColumns(ctx context.Context, tenantId, partyId int64, name, typ string, status int64,
			creditCode, region, address, remark sql.NullString, updatedBy int64) error
		// SoftDelete 软删（置 deleted_at）。
		SoftDelete(ctx context.Context, tenantId, partyId, updatedBy int64) error
	}

	customPartyModel struct {
		*defaultPartyModel
	}
)

// NewPartyModel returns a model for the database table.
func NewPartyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) PartyModel {
	return &customPartyModel{
		defaultPartyModel: newPartyModel(conn, c, opts...),
	}
}

func (m *customPartyModel) FindOne(ctx context.Context, tenantId, partyId int64) (*Party, error) {
	var resp Party
	query := fmt.Sprintf("select %s from %s where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		partyRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, partyId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customPartyModel) FindPage(ctx context.Context, tenantId int64, keyword, typ string, status int64, page, size int64) ([]*Party, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`name` like ? or `credit_code` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if typ != "" {
		// type 存 JSON 数组（SET 语义），包含匹配用 LIKE 兜底（规模小可接受）
		where += " and `type` like ?"
		args = append(args, "%\""+typ+"\"%")
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

	listQuery := fmt.Sprintf("select %s from %s where %s order by `party_id` desc limit ? offset ?",
		partyRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*Party
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customPartyModel) UpdateColumns(ctx context.Context, tenantId, partyId int64, name, typ string, status int64,
	creditCode, region, address, remark sql.NullString, updatedBy int64) error {
	query := fmt.Sprintf(`update %s set `+"`name`"+` = ?, `+"`type`"+` = ?, `+"`status`"+` = ?, `+"`credit_code`"+` = ?,
		`+"`region`"+` = ?, `+"`address`"+` = ?, `+"`remark`"+` = ?, `+"`updated_by`"+` = ?, `+"`updated_at`"+` = ?
		where `+"`party_id`"+` = ? and `+"`tenant_id`"+` = ? and `+"`deleted_at`"+` is null`, m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, name, typ, status, creditCode, region, address, remark,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), partyId, tenantId)
	return err
}

func (m *customPartyModel) SoftDelete(ctx context.Context, tenantId, partyId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `status` = 2, `updated_by` = ?, `updated_at` = ? where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), partyId, tenantId)
	return err
}
