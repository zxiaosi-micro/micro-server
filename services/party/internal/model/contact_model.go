// contact 表 custom 覆写（ADR-08）：租户过滤 + 显式列更新（口径同 party_model.go）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ContactModel = (*customContactModel)(nil)

type (
	// ContactModel 参与方联系人模型。
	ContactModel interface {
		// Insert 新建联系人（生成方法；is_default=1 时 Logic 层先调 ClearDefault 并保证同事务）。
		Insert(ctx context.Context, data *Contact) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, contactId int64) (*Contact, error)
		// ListByParty 参与方全部联系人（默认联系人置顶）。
		ListByParty(ctx context.Context, tenantId, partyId int64) ([]*Contact, error)
		// ClearDefault 清空参与方默认联系人标记（与 Insert 同事务由 Logic 层经 Conn.TransactCtx 编排）。
		ClearDefault(ctx context.Context, session sqlx.Session, tenantId, partyId, updatedBy int64) error
		// CountByParty 参与方联系人数（删除前校验）。
		CountByParty(ctx context.Context, tenantId, partyId int64) (int64, error)
		// SoftDelete 软删。
		SoftDelete(ctx context.Context, tenantId, contactId, updatedBy int64) error
	}

	customContactModel struct {
		*defaultContactModel
	}
)

// NewContactModel returns a model for the database table.
func NewContactModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContactModel {
	return &customContactModel{
		defaultContactModel: newContactModel(conn, c, opts...),
	}
}

func (m *customContactModel) FindOne(ctx context.Context, tenantId, contactId int64) (*Contact, error) {
	var resp Contact
	query := fmt.Sprintf("select %s from %s where `contact_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		contactRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, contactId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customContactModel) ListByParty(ctx context.Context, tenantId, partyId int64) ([]*Contact, error) {
	var list []*Contact
	query := fmt.Sprintf("select %s from %s where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `is_default` desc, `contact_id` asc",
		contactRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, partyId, tenantId)
	switch err {
	case nil:
		return list, nil
	case ErrNotFound:
		return nil, nil
	default:
		return nil, err
	}
}

func (m *customContactModel) ClearDefault(ctx context.Context, session sqlx.Session, tenantId, partyId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `is_default` = 0, `updated_by` = ?, `updated_at` = ? where `party_id` = ? and `tenant_id` = ? and `is_default` = 1 and `deleted_at` is null", m.table)
	_, err := session.ExecCtx(ctx, query,
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), partyId, tenantId)
	return err
}

func (m *customContactModel) CountByParty(ctx context.Context, tenantId, partyId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &n, query, partyId, tenantId)
	return n, err
}

func (m *customContactModel) SoftDelete(ctx context.Context, tenantId, contactId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `updated_at` = ? where `contact_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(),
		sql.NullInt64{Int64: updatedBy, Valid: updatedBy > 0}, time.Now(), contactId, tenantId)
	return err
}
