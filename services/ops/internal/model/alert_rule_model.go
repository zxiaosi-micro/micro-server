// alert_rule custom 覆写（ADR-08）：租户过滤显式携带 + 事务内插入 + 局部更新。
package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AlertRuleModel = (*customAlertRuleModel)(nil)

type (
	// AlertRuleModel 告警规则（FR-OPS-001）。
	AlertRuleModel interface {
		alertRuleModel
		// InsertTx 事务内插入（uk_rule_tenant_code 冲突 = 重码，调用方转 errRuleCodeDup）。
		InsertTx(ctx context.Context, session sqlx.Session, data *AlertRule) error
		FindOneScoped(ctx context.Context, tenantId, ruleId int64) (*AlertRule, error)
		FindOneByCodeScoped(ctx context.Context, tenantId int64, code string) (*AlertRule, error)
		ListEnabledByMetric(ctx context.Context, tenantId int64, metric string) ([]*AlertRule, error)
		ListPage(ctx context.Context, tenantId int64, metric, level string, enabled int64, limit, offset int) ([]*AlertRule, int64, error)
		UpdateFields(ctx context.Context, tenantId, ruleId int64, fields map[string]any) error
		SoftDelete(ctx context.Context, tenantId, ruleId int64) error
	}

	customAlertRuleModel struct {
		*defaultAlertRuleModel
		conn sqlx.SqlConn
	}
)

// NewAlertRuleModel 构造。
func NewAlertRuleModel(conn sqlx.SqlConn) AlertRuleModel {
	return &customAlertRuleModel{defaultAlertRuleModel: newAlertRuleModel(conn), conn: conn}
}

const alertRuleSoftFilter = "`deleted_at` is null"

func (m *customAlertRuleModel) InsertTx(ctx context.Context, session sqlx.Session, data *AlertRule) error {
	query := "insert into `alert_rule` (`rule_id`,`code`,`name`,`metric`,`comparator`,`threshold`,`duration_sec`,`level`,`suppress_window_sec`,`scope`,`enabled`,`tenant_id`,`created_by`) values (?,?,?,?,?,?,?,?,?,?,?,?,?)"
	_, err := session.ExecCtx(ctx, query,
		data.RuleId, data.Code, data.Name, data.Metric, data.Comparator, data.Threshold,
		data.DurationSec, data.Level, data.SuppressWindowSec, data.Scope, data.Enabled,
		data.TenantId, data.CreatedBy)
	return err
}

func (m *customAlertRuleModel) FindOneScoped(ctx context.Context, tenantId, ruleId int64) (*AlertRule, error) {
	var res AlertRule
	query := "select " + alertRuleRows + " from `alert_rule` where `rule_id` = ? and `tenant_id` = ? and " + alertRuleSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, ruleId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customAlertRuleModel) FindOneByCodeScoped(ctx context.Context, tenantId int64, code string) (*AlertRule, error) {
	var res AlertRule
	query := "select " + alertRuleRows + " from `alert_rule` where `tenant_id` = ? and `code` = ? and " + alertRuleSoftFilter
	if err := m.conn.QueryRowCtx(ctx, &res, query, tenantId, code); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customAlertRuleModel) ListEnabledByMetric(ctx context.Context, tenantId int64, metric string) ([]*AlertRule, error) {
	var list []*AlertRule
	query := "select " + alertRuleRows + " from `alert_rule` where `tenant_id` = ? and `metric` = ? and `enabled` = 1 and " + alertRuleSoftFilter
	if err := m.conn.QueryRowsCtx(ctx, &list, query, tenantId, metric); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customAlertRuleModel) ListPage(ctx context.Context, tenantId int64, metric, level string, enabled int64, limit, offset int) ([]*AlertRule, int64, error) {
	where := "`tenant_id` = ? and " + alertRuleSoftFilter
	args := []any{tenantId}
	if metric != "" {
		where += " and `metric` = ?"
		args = append(args, metric)
	}
	if level != "" {
		where += " and `level` = ?"
		args = append(args, level)
	}
	if enabled >= 0 {
		where += " and `enabled` = ?"
		args = append(args, enabled)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "select count(*) from `alert_rule` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*AlertRule
	args = append(args, limit, offset)
	query := "select " + alertRuleRows + " from `alert_rule` where " + where + " order by `created_at` desc limit ? offset ?"
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customAlertRuleModel) UpdateFields(ctx context.Context, tenantId, ruleId int64, fields map[string]any) error {
	set, args := buildSet(fields)
	query := "update `alert_rule` set " + set + " where `rule_id` = ? and `tenant_id` = ? and " + alertRuleSoftFilter
	args = append(args, ruleId, tenantId)
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

func (m *customAlertRuleModel) SoftDelete(ctx context.Context, tenantId, ruleId int64) error {
	query := "update `alert_rule` set `deleted_at` = now() where `rule_id` = ? and `tenant_id` = ? and " + alertRuleSoftFilter
	_, err := m.conn.ExecCtx(ctx, query, ruleId, tenantId)
	return err
}
