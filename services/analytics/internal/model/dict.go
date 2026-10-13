package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// MetricDict 指标字典行（S8-02 口径正本）。
type MetricDict struct {
	ID          int64     `db:"id"`           // 自增主键
	MetricCode  string    `db:"metric_code"`  // 指标码（对账 metric_code 与日报列同名）
	Name        string    `db:"name"`         // 指标显示名
	ReportType  string    `db:"report_type"`  // 所属报表类型（sales/inventory/warranty/sla/finance/station）
	Unit        string    `db:"unit"`         // 计量单位（单/笔/台/元…）
	Caliber     string    `db:"caliber"`      // 统计口径说明
	SourceEvent string    `db:"source_event"` // 来源事件类型
	Recon       int64     `db:"recon"`        // 是否参与日级对账（1=参与）
	Sort        int64     `db:"sort"`         // 展示排序
	Status      int64     `db:"status"`       // 状态（1=启用 0=停用）
	TenantId    int64     `db:"tenant_id"`    // 租户 ID（0=平台级字典）
	CreatedAt   time.Time `db:"created_at"`   // 创建时间
	UpdatedAt   time.Time `db:"updated_at"`   // 更新时间
}

// MetricDictModel 指标字典（report_metric_dict）的查询面。
type MetricDictModel interface {
	// List 字典查询（平台级 + 状态启用；report_type 空 = 全部）。
	List(ctx context.Context, reportType string) ([]*MetricDict, error)
}

type metricDictModel struct{ conn sqlx.SqlConn }

// NewMetricDictModel 创建指标字典模型（报表口径与对账范围的元数据正本）。
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
