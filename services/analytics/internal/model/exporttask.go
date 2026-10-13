package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ExportTask 异步导出任务（S8-03）。
type ExportTask struct {
	ID          int64     `db:"id"`           // 任务 ID（雪花，创建时外层生成）
	TenantId    int64     `db:"tenant_id"`    // 任务归属租户（worker 闭包按此重建 ctx）
	ReportType  string    `db:"report_type"`  // 导出报表类型（event_ledger/sales_daily 等）
	Format      string    `db:"format"`       // 导出格式（csv/xlsx/json）
	DateStart   time.Time `db:"date_start"`   // 导出窗口起始业务日（含）
	DateEnd     time.Time `db:"date_end"`     // 导出窗口结束业务日（含）
	Status      string    `db:"status"`       // 状态机：PENDING→RUNNING→DONE/FAILED
	FileId      int64     `db:"file_id"`      // 产物文件 ID（file 服务）
	FileName    string    `db:"file_name"`    // 产物文件名
	RowCount    int64     `db:"row_count"`    // 导出数据行数
	CostMs      int64     `db:"cost_ms"`      // 导出耗时（毫秒）
	Error       string    `db:"error"`        // 失败原因（DONE 时为空串）
	RequestedBy int64     `db:"requested_by"` // 发起人 uid
	CreatedAt   time.Time `db:"created_at"`   // 创建时间
	UpdatedAt   time.Time `db:"updated_at"`   // 更新时间
}

// ExportTaskModel 异步导出任务（export_task）的登记/查询/状态机推进面。
type ExportTaskModel interface {
	// InsertInTx 任务落库（与 export_requested Outbox 事件同事务）。
	InsertInTx(ctx context.Context, session sqlx.Session, t *ExportTask) error
	// FindOneScoped 租户 scoped 查询（服务面统一入口；tenantaudit 口径）。
	FindOneScoped(ctx context.Context, id, tid int64) (*ExportTask, error)
	// List 分页查询任务列表（status 为空查全部；size 上限 100）。
	List(ctx context.Context, tid int64, status string, page, size int) ([]*ExportTask, int64, error)
	// CasStatus 状态机推进（affected==0 = 已被并发 worker 抢走/终态，返回 false）。
	CasStatus(ctx context.Context, id int64, from, to string) (bool, error)
	// UpdateDone 任务完成回写（产物信息 + 计量，DONE 终态）。
	UpdateDone(ctx context.Context, id int64, fileID int64, fileName string, rowCount, costMs int64) error
	// UpdateFailed 任务失败回写（FAILED 终态，error 记原因）。
	UpdateFailed(ctx context.Context, id int64, msg string) error
}

type exportTaskModel struct{ conn sqlx.SqlConn }

// NewExportTaskModel 创建导出任务模型（CAS 推进保证并发 worker 只有一个接管）。
func NewExportTaskModel(conn sqlx.SqlConn) ExportTaskModel { return &exportTaskModel{conn: conn} }

func (m *exportTaskModel) InsertInTx(ctx context.Context, session sqlx.Session, t *ExportTask) error {
	_, err := session.ExecCtx(ctx,
		"INSERT INTO export_task (id, tenant_id, report_type, format, date_start, date_end, status, requested_by) VALUES (?,?,?,?,?,?,?,?)",
		t.ID, t.TenantId, t.ReportType, t.Format, t.DateStart, t.DateEnd, t.Status, t.RequestedBy)
	return err
}

const exportTaskCols = "id, tenant_id, report_type, format, date_start, date_end, status, file_id, file_name, row_count, cost_ms, error, requested_by, created_at, updated_at"

func (m *exportTaskModel) FindOneScoped(ctx context.Context, id, tid int64) (*ExportTask, error) {
	var t ExportTask
	err := m.conn.QueryRowCtx(ctx, &t,
		"SELECT "+exportTaskCols+" FROM export_task WHERE id = ? AND tenant_id = ?", id, tid)
	if err == sqlx.ErrNotFound {
		return nil, ErrNotFound
	}
	return &t, err
}

func (m *exportTaskModel) List(ctx context.Context, tid int64, status string, page, size int) ([]*ExportTask, int64, error) {
	where := "tenant_id = ?"
	args := []any{tid}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total,
		"SELECT COUNT(*) FROM export_task WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var rows []*ExportTask
	err := m.conn.QueryRowsCtx(ctx, &rows,
		"SELECT "+exportTaskCols+" FROM export_task WHERE "+where+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		append(args, size, (page-1)*size)...)
	return rows, total, err
}

func (m *exportTaskModel) CasStatus(ctx context.Context, id int64, from, to string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx,
		// tenantaudit:ignore 导出任务 CAS（id 全局主键，TaskRunner 闭包已按任务归属租户重建 ctx）
		"UPDATE export_task SET status = ? WHERE id = ? AND status = ?", to, id, from)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (m *exportTaskModel) UpdateDone(ctx context.Context, id int64, fileID int64, fileName string, rowCount, costMs int64) error {
	_, err := m.conn.ExecCtx(ctx,
		// tenantaudit:ignore 导出完成回写（id 全局主键，TaskRunner 闭包已按任务归属租户重建 ctx）
		"UPDATE export_task SET status = 'DONE', file_id = ?, file_name = ?, row_count = ?, cost_ms = ?, error = '' WHERE id = ?",
		fileID, fileName, rowCount, costMs, id)
	return err
}

func (m *exportTaskModel) UpdateFailed(ctx context.Context, id int64, msg string) error {
	_, err := m.conn.ExecCtx(ctx,
		// tenantaudit:ignore 导出失败回写（id 全局主键，TaskRunner 闭包已按任务归属租户重建 ctx）
		"UPDATE export_task SET status = 'FAILED', error = ? WHERE id = ?", msg, id)
	return err
}
