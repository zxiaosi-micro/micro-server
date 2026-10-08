// crm_record 表 custom 覆写（ADR-08）：跟进记录追加式（只 Insert/List，不提供改删）。

package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ CrmRecordModel = (*customCrmRecordModel)(nil)

type (
	// CrmRecordModel CRM 跟进记录模型（追加式）。
	CrmRecordModel interface {
		// Insert 追加跟进记录（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *CrmRecord) (sql.Result, error)
		// ListByParty 参与方全部跟进记录（新→旧）。
		ListByParty(ctx context.Context, tenantId, partyId int64) ([]*CrmRecord, error)
	}

	customCrmRecordModel struct {
		*defaultCrmRecordModel
	}
)

// NewCrmRecordModel returns a model for the database table.
func NewCrmRecordModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) CrmRecordModel {
	return &customCrmRecordModel{
		defaultCrmRecordModel: newCrmRecordModel(conn, c, opts...),
	}
}

func (m *customCrmRecordModel) ListByParty(ctx context.Context, tenantId, partyId int64) ([]*CrmRecord, error) {
	var list []*CrmRecord
	query := fmt.Sprintf("select %s from %s where `party_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `record_id` desc",
		crmRecordRows, m.table)
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
