// inspection_item custom 覆写：批量写入 + 异常项工单回填。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ InspectionItemModel = (*customInspectionItemModel)(nil)

type (
	// InspectionItemModel 巡检明细项（item.ticket_id 异常项直生成工单回填）。
	InspectionItemModel interface {
		inspectionItemModel
		// InsertTx 事务内写入。
		InsertTx(ctx context.Context, session sqlx.Session, data *InspectionItem) error
		// FillTicketTx 异常项生成工单后回填 ticket_id。
		FillTicketTx(ctx context.Context, session sqlx.Session, itemId, ticketId int64) error
		ListByTask(ctx context.Context, tenantId, taskId int64) ([]*InspectionItem, error)
	}

	customInspectionItemModel struct {
		*defaultInspectionItemModel
		conn sqlx.SqlConn
	}
)

// NewInspectionItemModel 构造。
func NewInspectionItemModel(conn sqlx.SqlConn) InspectionItemModel {
	return &customInspectionItemModel{defaultInspectionItemModel: newInspectionItemModel(conn), conn: conn}
}

func (m *customInspectionItemModel) InsertTx(ctx context.Context, session sqlx.Session, data *InspectionItem) error {
	query := "insert into `inspection_item` (`item_id`,`task_id`,`name`,`result`,`anomalous`,`tenant_id`) values (?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.ItemId, data.TaskId, data.Name, data.Result, data.Anomalous, data.TenantId)
	return err
}

func (m *customInspectionItemModel) FillTicketTx(ctx context.Context, session sqlx.Session, itemId, ticketId int64) error {
	_, err := session.ExecCtx(ctx,
		"update `inspection_item` set `ticket_id` = ? where `item_id` = ?", ticketId, itemId)
	return err
}

func (m *customInspectionItemModel) ListByTask(ctx context.Context, tenantId, taskId int64) ([]*InspectionItem, error) {
	var list []*InspectionItem
	query := "select " + inspectionItemRows + " from `inspection_item` where `task_id` = ? and `tenant_id` = ? order by `item_id` asc"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, taskId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}
