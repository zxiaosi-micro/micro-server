package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ExportTask 异步导出任务（S8-03）。
type ExportTask struct {
	ID          int64     `db:"id"`
	TenantId    int64     `db:"tenant_id"`
	ReportType  string    `db:"report_type"`
	Format      string    `db:"format"`
	DateStart   time.Time `db:"date_start"`
	DateEnd     time.Time `db:"date_end"`
	Status      string    `db:"status"`
	FileId      int64     `db:"file_id"`
	FileName    string    `db:"file_name"`
	RowCount    int64     `db:"row_count"`
	CostMs      int64     `db:"cost_ms"`
	Error       string    `db:"error"`
	RequestedBy int64     `db:"requested_by"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type ExportTaskModel interface {
	// InsertInTx 任务落库（与 export_requested Outbox 事件同事务）。
	InsertInTx(ctx context.Context, session sqlx.Session, t *ExportTask) error
	// FindOneScoped 租户 scoped 查询（服务面统一入口；tenantaudit 口径）。
	FindOneScoped(ctx context.Context, id, tid int64) (*ExportTask, error)
	List(ctx context.Context, tid int64, status string, page, size int) ([]*ExportTask, int64, error)
	// CasStatus 状态机推进（affected==0 = 已被并发 worker 抢走/终态，返回 false）。
	CasStatus(ctx context.Context, id int64, from, to string) (bool, error)
	UpdateDone(ctx context.Context, id int64, fileID int64, fileName string, rowCount, costMs int64) error
	UpdateFailed(ctx context.Context, id int64, msg string) error
}

type exportTaskModel struct{ conn sqlx.SqlConn }

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
		"UPDATE export_task SET status = ? WHERE id = ? AND status = ?", to, id, from)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (m *exportTaskModel) UpdateDone(ctx context.Context, id int64, fileID int64, fileName string, rowCount, costMs int64) error {
	_, err := m.conn.ExecCtx(ctx,
		"UPDATE export_task SET status = 'DONE', file_id = ?, file_name = ?, row_count = ?, cost_ms = ?, error = '' WHERE id = ?",
		fileID, fileName, rowCount, costMs, id)
	return err
}

func (m *exportTaskModel) UpdateFailed(ctx context.Context, id int64, msg string) error {
	_, err := m.conn.ExecCtx(ctx,
		"UPDATE export_task SET status = 'FAILED', error = ? WHERE id = ?", msg, id)
	return err
}
