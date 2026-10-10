// inspection 四表 custom 覆写：模板/计划/任务/明细（FR-OPS-007 + ADR-09）。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ InspectionTemplateModel = (*customInspectionTemplateModel)(nil)

type (
	// InspectionTemplateModel 巡检模板。
	InspectionTemplateModel interface {
		inspectionTemplateModel
		InsertTx(ctx context.Context, session sqlx.Session, data *InspectionTemplate) error
		FindOneScoped(ctx context.Context, tenantId, templateId int64) (*InspectionTemplate, error)
		// FindDefaultTx 租户默认模板（station_created 自动建计划用；无则建）。
		FindDefault(ctx context.Context, tenantId int64) (*InspectionTemplate, error)
		ListPage(ctx context.Context, tenantId int64, limit, offset int) ([]*InspectionTemplate, int64, error)
	}

	customInspectionTemplateModel struct {
		*defaultInspectionTemplateModel
		conn sqlx.SqlConn
	}
)

// NewInspectionTemplateModel 构造。
func NewInspectionTemplateModel(conn sqlx.SqlConn) InspectionTemplateModel {
	return &customInspectionTemplateModel{defaultInspectionTemplateModel: newInspectionTemplateModel(conn), conn: conn}
}

const inspectionTemplateSoftFilter = "`deleted_at` is null"

// DefaultInspectionSchema 默认日巡模板 schema（station_created 自动建计划引用）。
const DefaultInspectionSchema = `[{"name":"外观检查","type":"bool","required":true},{"name":"连接紧固","type":"bool","required":true},{"name":"异味异响","type":"bool","required":true},{"name":"环境温度","type":"number","required":false},{"name":"其他备注","type":"text","required":false}]`

func (m *customInspectionTemplateModel) InsertTx(ctx context.Context, session sqlx.Session, data *InspectionTemplate) error {
	query := "insert into `inspection_template` (`template_id`,`name`,`items_schema`,`tenant_id`,`created_by`) values (?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query, data.TemplateId, data.Name, data.ItemsSchema, data.TenantId, data.CreatedBy)
	return err
}

func (m *customInspectionTemplateModel) FindOneScoped(ctx context.Context, tenantId, templateId int64) (*InspectionTemplate, error) {
	var res InspectionTemplate
	query := "select " + inspectionTemplateRows + " from `inspection_template` where `template_id` = ? and `tenant_id` = ? and " + inspectionTemplateSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, templateId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInspectionTemplateModel) FindDefault(ctx context.Context, tenantId int64) (*InspectionTemplate, error) {
	var res InspectionTemplate
	query := "select " + inspectionTemplateRows + " from `inspection_template` where `tenant_id` = ? and `name` = '默认日巡模板' and " + inspectionTemplateSoftFilter + " limit 1"
	if err := m.conn.QueryRowCtx(ctx, &res, query, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customInspectionTemplateModel) ListPage(ctx context.Context, tenantId int64, limit, offset int) ([]*InspectionTemplate, int64, error) {
	where := "`tenant_id` = ? and " + inspectionTemplateSoftFilter
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `inspection_template` where "+where, tenantId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*InspectionTemplate
	query := "select " + inspectionTemplateRows + " from `inspection_template` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, limit, offset); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
