package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ReconDiff 日级对账差异（S8-04；修复只走补投递重放，E10 禁裸删）。
type ReconDiff struct {
	ID          int64      `db:"id"`
	TenantId    int64      `db:"tenant_id"`
	ReconDate   time.Time  `db:"recon_date"`
	BizDate     time.Time  `db:"biz_date"`
	MetricCode  string     `db:"metric_code"`
	SourceCount int64      `db:"source_count"`
	ModelCount  int64      `db:"model_count"`
	DiffCount   int64      `db:"diff_count"`
	Status      string     `db:"status"`
	ResolvedAt  *time.Time `db:"resolved_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// ReconState 对账执行状态（cron 幂等防重）。
type ReconState struct {
	BizDate   time.Time  `db:"biz_date"`
	Status    string     `db:"status"`
	DiffTotal int64      `db:"diff_total"`
	Notified  int64      `db:"notified"`
	RunAt     *time.Time `db:"run_at"`
}

type ReconModel interface {
	// UpsertDiff 登记差异（uk tenant+metric+bizdate 幂等；OPEN 且与上次值一致则不重复告警）。
	UpsertDiff(ctx context.Context, d *ReconDiff) error
	// ResolveDiff 差异补齐后关闭（CAS：OPEN → RESOLVED）。
	ResolveDiff(ctx context.Context, tid int64, metricCode string, bizDate time.Time) error
	ListOpen(ctx context.Context, reconDate time.Time) ([]*ReconDiff, error)
	CountOpen(ctx context.Context, bizDate time.Time) (int64, error)

	// UpsertState 对账状态登记（recon_state 幂等）。
	UpsertState(ctx context.Context, bizDate time.Time, status string, diffTotal int64) error
	GetState(ctx context.Context, bizDate time.Time) (*ReconState, error)
	MarkNotified(ctx context.Context, bizDate time.Time) error
}

type reconModel struct{ conn sqlx.SqlConn }

func NewReconModel(conn sqlx.SqlConn) ReconModel { return &reconModel{conn: conn} }

func (m *reconModel) UpsertDiff(ctx context.Context, d *ReconDiff) error {
	res, err := m.conn.ExecCtx(ctx,
		"INSERT INTO recon_diff (tenant_id, recon_date, biz_date, metric_code, source_count, model_count, diff_count, status) "+
			"VALUES (?,?,?,?,?,?,?,'OPEN') "+
			"ON DUPLICATE KEY UPDATE recon_date = VALUES(recon_date), source_count = VALUES(source_count), model_count = VALUES(model_count), diff_count = VALUES(diff_count), status = 'OPEN', resolved_at = NULL",
		d.TenantId, d.ReconDate, d.BizDate, d.MetricCode, d.SourceCount, d.ModelCount, d.DiffCount)
	if err != nil {
		return err
	}
	if d.ID == 0 {
		if id, ierr := res.LastInsertId(); ierr == nil {
			d.ID = id
		}
	}
	return nil
}

func (m *reconModel) ResolveDiff(ctx context.Context, tid int64, metricCode string, bizDate time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"UPDATE recon_diff SET status = 'RESOLVED', resolved_at = NOW() WHERE tenant_id = ? AND metric_code = ? AND biz_date = ? AND status = 'OPEN'",
		tid, metricCode, bizDate)
	return err
}

func (m *reconModel) ListOpen(ctx context.Context, reconDate time.Time) ([]*ReconDiff, error) {
	var rows []*ReconDiff
	err := m.conn.QueryRowsCtx(ctx, &rows,
		// tenantaudit:ignore 对账开放差异清单（daily_recon 平台级批次推进，跨租户）
		"SELECT id, tenant_id, recon_date, biz_date, metric_code, source_count, model_count, diff_count, status, resolved_at, created_at, updated_at FROM recon_diff WHERE status = 'OPEN' AND recon_date = ?",
		reconDate)
	return rows, err
}

func (m *reconModel) CountOpen(ctx context.Context, bizDate time.Time) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n,
		// tenantaudit:ignore 对账开放差异计数（daily_recon 平台级批次推进，跨租户）
		"SELECT COUNT(*) FROM recon_diff WHERE status = 'OPEN' AND biz_date = ?", bizDate)
	return n, err
}

func (m *reconModel) UpsertState(ctx context.Context, bizDate time.Time, status string, diffTotal int64) error {
	_, err := m.conn.ExecCtx(ctx,
		"INSERT INTO recon_state (biz_date, status, diff_total, run_at) VALUES (?,?,?,NOW()) "+
			"ON DUPLICATE KEY UPDATE status = VALUES(status), diff_total = VALUES(diff_total), run_at = NOW()",
		bizDate, status, diffTotal)
	return err
}

func (m *reconModel) GetState(ctx context.Context, bizDate time.Time) (*ReconState, error) {
	var s ReconState
	err := m.conn.QueryRowCtx(ctx, &s,
		"SELECT biz_date, status, diff_total, notified, run_at FROM recon_state WHERE biz_date = ?", bizDate)
	if err == sqlx.ErrNotFound {
		return nil, ErrNotFound
	}
	return &s, err
}

func (m *reconModel) MarkNotified(ctx context.Context, bizDate time.Time) error {
	_, err := m.conn.ExecCtx(ctx,
		"UPDATE recon_state SET notified = 1 WHERE biz_date = ?", bizDate)
	return err
}
