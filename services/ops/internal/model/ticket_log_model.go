// ticket_log custom 覆写：追加写入 + 按工单查询。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TicketLogModel = (*customTicketLogModel)(nil)

type (
	// TicketLogModel 工单日志（状态迁移留痕；operator_uid=0 表示系统）。
	TicketLogModel interface {
		ticketLogModel
		// InsertTx 事务内追加（与工单迁移同事务）。
		InsertTx(ctx context.Context, session sqlx.Session, data *TicketLog) error
		ListByTicket(ctx context.Context, tenantId, ticketId int64) ([]*TicketLog, error)
	}

	customTicketLogModel struct {
		*defaultTicketLogModel
		conn sqlx.SqlConn
	}
)

// NewTicketLogModel 构造。
func NewTicketLogModel(conn sqlx.SqlConn) TicketLogModel {
	return &customTicketLogModel{defaultTicketLogModel: newTicketLogModel(conn), conn: conn}
}

func (m *customTicketLogModel) InsertTx(ctx context.Context, session sqlx.Session, data *TicketLog) error {
	query := "insert into `ticket_log` (`log_id`,`ticket_id`,`action`,`from_status`,`to_status`,`operator_uid`,`note`,`tenant_id`) values (?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.LogId, data.TicketId, data.Action, data.FromStatus, data.ToStatus,
		data.OperatorUid, data.Note, data.TenantId)
	return err
}

func (m *customTicketLogModel) ListByTicket(ctx context.Context, tenantId, ticketId int64) ([]*TicketLog, error) {
	var list []*TicketLog
	query := "select " + ticketLogRows + " from `ticket_log` where `ticket_id` = ? and `tenant_id` = ? order by `log_id` asc"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, ticketId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}
