// contract_template / contract_target / sla_strategy / contract_sla / claim 表 custom 覆写（ADR-08）。

package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— contract_template ——

var _ ContractTemplateModel = (*customContractTemplateModel)(nil)

type (
	// ContractTemplateModel 合同模板模型（变量+body，L2）。
	ContractTemplateModel interface {
		contractTemplateModel
		// InsertTx 模板落库。
		InsertTx(ctx context.Context, data *ContractTemplate) error
		// FindOneScoped 租户过滤取模板。
		FindOneScoped(ctx context.Context, tenantId, templateId int64) (*ContractTemplate, error)
	}

	customContractTemplateModel struct {
		*defaultContractTemplateModel
		conn sqlx.SqlConn
	}
)

// NewContractTemplateModel returns a model for the database table.
func NewContractTemplateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContractTemplateModel {
	return &customContractTemplateModel{
		defaultContractTemplateModel: newContractTemplateModel(conn, c, opts...),
		conn:                         conn,
	}
}

func (m *customContractTemplateModel) InsertTx(ctx context.Context, data *ContractTemplate) error {
	query := "insert into `contract_template` (`template_id`, `code`, `name`, `contract_type`, `variables_json`, `body`, `status`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := m.conn.ExecCtx(ctx, query, data.TemplateId, data.Code, data.Name, data.ContractType,
		data.VariablesJson, data.Body, data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customContractTemplateModel) FindOneScoped(ctx context.Context, tenantId, templateId int64) (*ContractTemplate, error) {
	var res ContractTemplate
	query := "select " + contractTemplateRows + " from `contract_template` where `template_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, templateId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

// —— contract_target ——

var _ ContractTargetModel = (*customContractTargetModel)(nil)

type (
	// ContractTargetModel 合同关联模型（订单/客户/场站，归档五要素之「关联」）。
	ContractTargetModel interface {
		contractTargetModel
		// InsertTx 事务内写关联。
		InsertTx(ctx context.Context, session sqlx.Session, data *ContractTarget) error
		// ListByContract 合同关联列表。
		ListByContract(ctx context.Context, tenantId, contractId int64) ([]*ContractTarget, error)
	}

	customContractTargetModel struct {
		*defaultContractTargetModel
		conn sqlx.SqlConn
	}
)

// NewContractTargetModel returns a model for the database table.
func NewContractTargetModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContractTargetModel {
	return &customContractTargetModel{
		defaultContractTargetModel: newContractTargetModel(conn, c, opts...),
		conn:                       conn,
	}
}

func (m *customContractTargetModel) InsertTx(ctx context.Context, session sqlx.Session, data *ContractTarget) error {
	query := "insert into `contract_target` (`target_rec_id`, `contract_id`, `target_type`, `target_pk`, `target_no`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.TargetRecId, data.ContractId, data.TargetType,
		data.TargetPk, data.TargetNo, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customContractTargetModel) ListByContract(ctx context.Context, tenantId, contractId int64) ([]*ContractTarget, error) {
	var list []*ContractTarget
	query := "select " + contractTargetRows + " from `contract_target` where `contract_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `target_rec_id`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, contractId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

// —— sla_strategy ——

var _ SlaStrategyModel = (*customSlaStrategyModel)(nil)

type (
	// SlaStrategyModel SLA 策略模型（FR-CTR-007）。
	SlaStrategyModel interface {
		slaStrategyModel
		// InsertTx 策略落库。
		InsertTx(ctx context.Context, data *SlaStrategy) error
		// FindOneScoped 租户过滤取策略。
		FindOneScoped(ctx context.Context, tenantId, strategyId int64) (*SlaStrategy, error)
		// ListAll 策略全量（管理面）。
		ListAll(ctx context.Context, tenantId int64) ([]*SlaStrategy, error)
	}

	customSlaStrategyModel struct {
		*defaultSlaStrategyModel
		conn sqlx.SqlConn
	}
)

// NewSlaStrategyModel returns a model for the database table.
func NewSlaStrategyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) SlaStrategyModel {
	return &customSlaStrategyModel{
		defaultSlaStrategyModel: newSlaStrategyModel(conn, c, opts...),
		conn:                    conn,
	}
}

func (m *customSlaStrategyModel) InsertTx(ctx context.Context, data *SlaStrategy) error {
	query := "insert into `sla_strategy` (`strategy_id`, `code`, `name`, `level`, `response_minutes`, `resolve_minutes`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := m.conn.ExecCtx(ctx, query, data.StrategyId, data.Code, data.Name, data.Level,
		data.ResponseMinutes, data.ResolveMinutes, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customSlaStrategyModel) ListAll(ctx context.Context, tenantId int64) ([]*SlaStrategy, error) {
	var list []*SlaStrategy
	query := "select " + slaStrategyRows + " from `sla_strategy` where `tenant_id` = ? and `deleted_at` is null order by `strategy_id`"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customSlaStrategyModel) FindOneScoped(ctx context.Context, tenantId, strategyId int64) (*SlaStrategy, error) {
	var res SlaStrategy
	query := "select " + slaStrategyRows + " from `sla_strategy` where `strategy_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, strategyId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

// —— contract_sla ——

var _ ContractSlaModel = (*customContractSlaModel)(nil)

type (
	// ContractSlaModel 合同 SLA 绑定模型（绑定即快照冻结）。
	ContractSlaModel interface {
		contractSlaModel
		// InsertTx 绑定落库（contract_id UK——一合同一策略绑定）。
		InsertTx(ctx context.Context, session sqlx.Session, data *ContractSla) error
	}

	customContractSlaModel struct {
		*defaultContractSlaModel
		conn sqlx.SqlConn
	}
)

// NewContractSlaModel returns a model for the database table.
func NewContractSlaModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContractSlaModel {
	return &customContractSlaModel{
		defaultContractSlaModel: newContractSlaModel(conn, c, opts...),
		conn:                    conn,
	}
}

func (m *customContractSlaModel) InsertTx(ctx context.Context, session sqlx.Session, data *ContractSla) error {
	query := "insert into `contract_sla` (`bind_id`, `contract_id`, `strategy_id`, `snapshot`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.BindId, data.ContractId, data.StrategyId,
		data.Snapshot, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

// —— claim ——

var _ ClaimModel = (*customClaimModel)(nil)

type (
	// ClaimModel 索赔模型（申请/审核/结算，FR-CTR-008）。
	ClaimModel interface {
		claimModel
		// InsertTx 索赔落库。
		InsertTx(ctx context.Context, session sqlx.Session, data *Claim) error
		// FindOneByNo 租户过滤按索赔号取。
		FindOneByNo(ctx context.Context, tenantId int64, claimNo string) (*Claim, error)
		// FindOneByNoForUpdateTx 行锁取（审批/结算并发入口）。
		FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, claimNo string) (*Claim, error)
		// CASStatusTx 状态机（APPLYING/APPROVED/REJECTED/SETTLED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, from []string, to string) error
		// MarkSettledTx 结算回写。
		MarkSettledTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, settleType string, amount float64, at time.Time) error
		// MarkRejectedTx 驳回回写。
		MarkRejectedTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, remark string) error
		// ListPage 索赔列表。
		ListPage(ctx context.Context, tenantId int64, status string, page, size int64) ([]*Claim, int64, error)
	}

	customClaimModel struct {
		*defaultClaimModel
		conn sqlx.SqlConn
	}
)

// NewClaimModel returns a model for the database table.
func NewClaimModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ClaimModel {
	return &customClaimModel{
		defaultClaimModel: newClaimModel(conn, c, opts...),
		conn:              conn,
	}
}

func (m *customClaimModel) InsertTx(ctx context.Context, session sqlx.Session, data *Claim) error {
	query := "insert into `claim` (`claim_id`, `claim_no`, `warranty_id`, `warranty_no`, `type`, `description`, `status`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ClaimId, data.ClaimNo, data.WarrantyId, data.WarrantyNo,
		data.Type, data.Description, data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customClaimModel) FindOneByNo(ctx context.Context, tenantId int64, claimNo string) (*Claim, error) {
	var res Claim
	query := "select " + claimRows + " from `claim` where `claim_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, claimNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customClaimModel) FindOneByNoForUpdateTx(ctx context.Context, session sqlx.Session, tenantId int64, claimNo string) (*Claim, error) {
	var res Claim
	query := "select " + claimRows + " from `claim` where `claim_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1 for update"
	if err := session.QueryRowCtx(ctx, &res, query, claimNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customClaimModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, claimId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `claim` set `status` = ? where `claim_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customClaimModel) MarkSettledTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, settleType string, amount float64, at time.Time) error {
	query := "update `claim` set `status` = 'SETTLED', `settle_type` = ?, `amount` = ?, `settle_at` = ? where `claim_id` = ? and `tenant_id` = ? and `status` = 'APPROVED'"
	res, err := session.ExecCtx(ctx, query, settleType, amount, at, claimId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customClaimModel) ListPage(ctx context.Context, tenantId int64, status string, page, size int64) ([]*Claim, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `claim` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Claim
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+claimRows+" from `claim` where "+where+" order by `claim_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customClaimModel) MarkRejectedTx(ctx context.Context, session sqlx.Session, tenantId, claimId int64, remark string) error {
	query := "update `claim` set `status` = 'REJECTED', `reject_remark` = ? where `claim_id` = ? and `tenant_id` = ? and `status` = 'APPLYING'"
	res, err := session.ExecCtx(ctx, query, remark, claimId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}
