package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// EventEntry 读模型事件台账（业务键幂等正本）。
type EventEntry struct {
	ID         int64     `db:"id"`          // 自增主键（导出按 id 升序流式扫描游标）
	EventID    string    `db:"event_id"`    // 事件全局唯一 ID（与 event_dedup 同源）
	EventType  string    `db:"event_type"`  // 事件类型（点号命名）
	Topic      string    `db:"topic"`       // 来源 Kafka topic
	TenantId   int64     `db:"tenant_id"`   // 租户 ID
	BizKey     string    `db:"biz_key"`     // 业务幂等键（uk tenant+event_type+biz_key，重复事件整单跳过）
	StatDate   time.Time `db:"stat_date"`   // 归档业务日（导出与补投递的时间口径）
	Payload    string    `db:"payload"`     // 事件负载 JSON 快照
	ConsumedAt time.Time `db:"consumed_at"` // 台账落库时间
}

// EventEntryModel 读模型事件台账（report_event_entry）的写入与明细导出面。
type EventEntryModel interface {
	// InsertInTx 台账落库（uk_biz 幂等：重复返回 ErrDupBiz，调用方整单跳过）。
	InsertInTx(ctx context.Context, session sqlx.Session, e *EventEntry) error
	// ListRange 台账明细（event_ledger 导出数据源；按 id 升序流式扫描）。
	ListRange(ctx context.Context, tid int64, start, end time.Time, lastID int64, limit int) ([]*EventEntry, error)
}

type eventEntryModel struct{ conn sqlx.SqlConn }

// NewEventEntryModel 创建事件台账模型（幂等第一层：uk_biz）。
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
