package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// EventEntry 读模型事件台账（业务键幂等正本）。
type EventEntry struct {
	ID         int64     `db:"id"`
	EventID    string    `db:"event_id"`
	EventType  string    `db:"event_type"`
	Topic      string    `db:"topic"`
	TenantId   int64     `db:"tenant_id"`
	BizKey     string    `db:"biz_key"`
	StatDate   time.Time `db:"stat_date"`
	Payload    string    `db:"payload"`
	ConsumedAt time.Time `db:"consumed_at"`
}

type EventEntryModel interface {
	// InsertInTx 台账落库（uk_biz 幂等：重复返回 ErrDupBiz，调用方整单跳过）。
	InsertInTx(ctx context.Context, session sqlx.Session, e *EventEntry) error
	// ListRange 台账明细（event_ledger 导出数据源；按 id 升序流式扫描）。
	ListRange(ctx context.Context, tid int64, start, end time.Time, lastID int64, limit int) ([]*EventEntry, error)
}

type eventEntryModel struct{ conn sqlx.SqlConn }

func NewEventEntryModel(conn sqlx.SqlConn) EventEntryModel { return &eventEntryModel{conn: conn} }

func (m *eventEntryModel) InsertInTx(ctx context.Context, session sqlx.Session, e *EventEntry) error {
	res, err := session.ExecCtx(ctx,
		"INSERT INTO report_event_entry (event_id, event_type, topic, tenant_id, biz_key, stat_date, payload) VALUES (?,?,?,?,?,?,?)",
		e.EventID, e.EventType, e.Topic, e.TenantId, e.BizKey, e.StatDate, e.Payload)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		e.ID = id
	}
	return nil
}

func (m *eventEntryModel) ListRange(ctx context.Context, tid int64, start, end time.Time, lastID int64, limit int) ([]*EventEntry, error) {
	var rows []*EventEntry
	err := m.conn.QueryRowsCtx(ctx, &rows,
		"SELECT id, event_id, event_type, topic, tenant_id, biz_key, stat_date, payload, consumed_at FROM report_event_entry WHERE tenant_id = ? AND stat_date >= ? AND stat_date <= ? AND id > ? ORDER BY id LIMIT ?",
		tid, start, end, lastID, limit)
	return rows, err
}
