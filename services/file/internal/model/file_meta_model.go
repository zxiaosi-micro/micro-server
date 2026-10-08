// file_meta 表 custom 覆写（ADR-08）：租户过滤 + NoCache 查询（口径同 party/catalog）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ FileMetaModel = (*customFileMetaModel)(nil)

type (
	// FileMetaModel 文件元数据模型。
	FileMetaModel interface {
		// Insert 新建元数据（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *FileMeta) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, fileId int64) (*FileMeta, error)
		// FindPage 元数据列表（bizType/bizId 空 = 全部）。
		FindPage(ctx context.Context, tenantId int64, bizType, bizId string, page, size int64) ([]*FileMeta, int64, error)
		// SoftDelete 软删（对象保留，回收属运维流程）。
		SoftDelete(ctx context.Context, tenantId, fileId, updatedBy int64) error
	}

	customFileMetaModel struct {
		*defaultFileMetaModel
	}
)

// NewFileMetaModel returns a model for the database table.
func NewFileMetaModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) FileMetaModel {
	return &customFileMetaModel{
		defaultFileMetaModel: newFileMetaModel(conn, c, opts...),
	}
}

func (m *customFileMetaModel) FindOne(ctx context.Context, tenantId, fileId int64) (*FileMeta, error) {
	var resp FileMeta
	query := fmt.Sprintf("select %s from %s where `file_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1",
		fileMetaRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, fileId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customFileMetaModel) FindPage(ctx context.Context, tenantId int64, bizType, bizId string, page, size int64) ([]*FileMeta, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if bizType != "" {
		where += " and `biz_type` = ?"
		args = append(args, bizType)
	}
	if bizId != "" {
		where += " and `biz_id` = ?"
		args = append(args, bizId)
	}

	var total int64
	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	if err := m.QueryRowNoCacheCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	listQuery := fmt.Sprintf("select %s from %s where %s order by `file_id` desc limit ? offset ?",
		fileMetaRows, m.table, where)
	args = append(args, size, (page-1)*size)
	var list []*FileMeta
	if err := m.QueryRowsNoCacheCtx(ctx, &list, listQuery, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customFileMetaModel) SoftDelete(ctx context.Context, tenantId, fileId, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `deleted_at` = ?, `updated_by` = ?, `updated_at` = ? where `file_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(), toNullInt64(updatedBy), time.Now(), fileId, tenantId)
	return err
}

func toNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}
