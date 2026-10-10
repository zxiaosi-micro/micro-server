package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// MetricDict 指标字典行（S8-02 口径正本）。
type MetricDict struct {
	ID          int64     `db:"id"`
	MetricCode  string    `db:"metric_code"`
	Name        string    `db:"name"`
	ReportType  string    `db:"report_type"`
	Unit        string    `db:"unit"`
	Caliber     string    `db:"caliber"`
	SourceEvent string    `db:"source_event"`
	Recon       int64     `db:"recon"`
	Sort        int64     `db:"sort"`
	Status      int64     `db:"status"`
	TenantId    int64     `db:"tenant_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type MetricDictModel interface {
	List(ctx context.Context, reportType string) ([]*MetricDict, error)
}

type metricDictModel struct{ conn sqlx.SqlConn }

func NewMetricDictModel(conn sqlx.SqlConn) MetricDictModel { return &metricDictModel{conn: conn} }

// List 字典查询（平台级 + 状态启用；report_type 空 = 全部）。
func (m *metricDictModel) List(ctx context.Context, reportType string) ([]*MetricDict, error) {
	query := "SELECT id, metric_code, name, report_type, unit, caliber, source_event, recon, sort, status, tenant_id, created_at, updated_at FROM report_metric_dict WHERE status = 1"
	var args []any
	if reportType != "" {
		query += " AND report_type = ?"
		args = append(args, reportType)
	}
	query += " ORDER BY report_type, sort"
	var rows []*MetricDict
	err := m.conn.QueryRowsCtx(ctx, &rows, query, args...) // 聚合/扫描查询禁缓存（E14）
	return rows, err
}
