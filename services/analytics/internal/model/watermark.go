package model

import (
	"context"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Watermark 消费水位（平台级，无租户维度）。
type Watermark struct {
	ID             int64      `db:"id"`              // 自增主键
	Topic          string     `db:"topic"`           // 消费的 Kafka topic
	ConsumerGroup  string     `db:"consumer_group"`  // 消费组（analytics-read-v{N}，随重建递增）
	LastEventID    string     `db:"last_event_id"`   // 已应用到的最后事件 ID（重放断点）
	LastStatDate   *time.Time `db:"last_stat_date"`  // 已应用到的最后业务日
	ProcessedCount int64      `db:"processed_count"` // 累计应用事件数
	UpdatedAt      time.Time  `db:"updated_at"`      // 最近推进时间
}

// WatermarkModel 消费水位（report_watermark）的推进与查看面。
type WatermarkModel interface {
	// BumpInTx 事件应用成功后推进水位（与应用同事务）。
	BumpInTx(ctx context.Context, s sqlx.Session, topic, group, eventID string, statDate time.Time) error
	// List 全部水位行（运维查看消费进度）。
	List(ctx context.Context) ([]*Watermark, error)
}

type watermarkModel struct{ conn sqlx.SqlConn }

// NewWatermarkModel 创建消费水位模型（水位推进与事件应用同事务，保证重放一致）。
func NewWatermarkModel(conn sqlx.SqlConn) WatermarkModel { return &watermarkModel{conn: conn} }

func (m *watermarkModel) BumpInTx(ctx context.Context, s sqlx.Session, topic, group, eventID string, statDate time.Time) error {
	_, err := s.ExecCtx(ctx,
		"INSERT INTO report_watermark (topic, consumer_group, last_event_id, last_stat_date, processed_count) VALUES (?,?,?,?,1) "+
			"ON DUPLICATE KEY UPDATE consumer_group = VALUES(consumer_group), last_event_id = VALUES(last_event_id), last_stat_date = VALUES(last_stat_date), processed_count = processed_count + 1",
		topic, group, eventID, statDate)
	return err
}

func (m *watermarkModel) List(ctx context.Context) ([]*Watermark, error) {
	var rows []*Watermark
	err := m.conn.QueryRowsCtx(ctx, &rows,
		"SELECT id, topic, consumer_group, last_event_id, last_stat_date, processed_count, updated_at FROM report_watermark ORDER BY topic")
	return rows, err
}

// ---------- 元数据（消费组版本号） ----------

// MetaModel 读模型元数据（report_meta 的 rebuild_version 消费组版本号）。
type MetaModel interface {
	// GetVersion 读 rebuild_version（缺省 "1"）。
	GetVersion(ctx context.Context) (string, error)
	// NextVersion 版本 +1（重建时调用），返回新版本号。
	NextVersion(ctx context.Context) (int64, error)
}

type metaModel struct{ conn sqlx.SqlConn }

// NewMetaModel 创建元数据模型（RebuildReadModel 时版本 +1 → 新消费组从头回放）。
func NewMetaModel(conn sqlx.SqlConn) MetaModel { return &metaModel{conn: conn} }

func (m *metaModel) GetVersion(ctx context.Context) (string, error) {
	var v string
	err := m.conn.QueryRowCtx(ctx, &v, "SELECT v FROM report_meta WHERE k = 'rebuild_version'")
	if err == sqlx.ErrNotFound {
		return "1", nil
	}
	return v, err
}

func (m *metaModel) NextVersion(ctx context.Context) (int64, error) {
	cur, err := m.GetVersion(ctx)
	if err != nil {
		return 0, err
	}
	next, _ := strconv.Atoi(cur)
	if next < 1 {
		next = 1
	}
	next++
	_, err = m.conn.ExecCtx(ctx,
		"INSERT INTO report_meta (k, v) VALUES ('rebuild_version', ?) ON DUPLICATE KEY UPDATE v = VALUES(v)",
		strconv.Itoa(next))
	return int64(next), err
}
