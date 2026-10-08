// contract_db 各表 custom 覆写（ADR-08）：租户过滤 + 事务方法 + CAS 状态机。
// contract/contract_file/warranty 三表是核心写面；其余表走简单 custom。

package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— contract ——

var _ ContractModel = (*customContractModel)(nil)

type (
	// ContractModel 合同模型（草稿→生效→归档）。
	ContractModel interface {
		contractModel
		// InsertTx 事务内建合同（Saga 步骤5）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Contract) error
		// FindOneScoped 租户过滤取合同。
		FindOneScoped(ctx context.Context, tenantId, contractId int64) (*Contract, error)
		// FindOneByNo 租户过滤按合同号取。
		FindOneByNo(ctx context.Context, tenantId int64, contractNo string) (*Contract, error)
		// FindOneByOrderNo 幂等锚点：同订单合同（CreateFromOrder 重放）。
		FindOneByOrderNo(ctx context.Context, tenantId int64, orderNo string) (*Contract, error)
		// CASStatusTx 状态机（DRAFT/ACTIVE/ARCHIVED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, from []string, to string) error
		// MarkEffectiveTx 生效回写。
		MarkEffectiveTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, at time.Time) error
		// MarkArchivedTx 归档回写。
		MarkArchivedTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, at time.Time) error
		// ListPage 合同列表。
		ListPage(ctx context.Context, tenantId int64, keyword, contractType, status string, page, size int64) ([]*Contract, int64, error)
	}

	customContractModel struct {
		*defaultContractModel
		conn sqlx.SqlConn
	}
)

// NewContractModel returns a model for the database table.
func NewContractModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContractModel {
	return &customContractModel{
		defaultContractModel: newContractModel(conn, c, opts...),
		conn:                 conn,
	}
}

func (m *customContractModel) InsertTx(ctx context.Context, session sqlx.Session, data *Contract) error {
	query := "insert into `contract` (`contract_id`, `contract_no`, `type`, `status`, `name`, `template_id`, `amount`, `buyer_party_id`, `order_no`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ContractId, data.ContractNo, data.Type, data.Status,
		data.Name, data.TemplateId, data.Amount, data.BuyerPartyId, data.OrderNo, data.Remark,
		data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customContractModel) FindOneScoped(ctx context.Context, tenantId, contractId int64) (*Contract, error) {
	var res Contract
	query := "select " + contractRows + " from `contract` where `contract_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, contractId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customContractModel) FindOneByNo(ctx context.Context, tenantId int64, contractNo string) (*Contract, error) {
	var res Contract
	query := "select " + contractRows + " from `contract` where `contract_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, contractNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customContractModel) FindOneByOrderNo(ctx context.Context, tenantId int64, orderNo string) (*Contract, error) {
	var res Contract
	query := "select " + contractRows + " from `contract` where `order_no` = ? and `tenant_id` = ? and `deleted_at` is null limit 1"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, orderNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customContractModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, contractId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `contract` set `status` = ? where `contract_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customContractModel) MarkEffectiveTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, at time.Time) error {
	query := "update `contract` set `status` = 'ACTIVE', `effective_at` = ? where `contract_id` = ? and `tenant_id` = ? and `status` = 'DRAFT'"
	res, err := session.ExecCtx(ctx, query, at, contractId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customContractModel) MarkArchivedTx(ctx context.Context, session sqlx.Session, tenantId, contractId int64, at time.Time) error {
	query := "update `contract` set `status` = 'ARCHIVED', `archived_at` = ? where `contract_id` = ? and `tenant_id` = ? and `status` = 'ACTIVE'"
	res, err := session.ExecCtx(ctx, query, at, contractId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customContractModel) ListPage(ctx context.Context, tenantId int64, keyword, contractType, status string, page, size int64) ([]*Contract, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and (`contract_no` like ? or `name` like ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if contractType != "" {
		where += " and `type` = ?"
		args = append(args, contractType)
	}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `contract` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Contract
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+contractRows+" from `contract` where "+where+" order by `contract_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// —— contract_file ——

var _ ContractFileModel = (*customContractFileModel)(nil)

type (
	// ContractFileModel 归档件模型（五要素：上传人/版本/关联/状态/受控下载）。
	ContractFileModel interface {
		contractFileModel
		// InsertTx 事务内归档（版本 UK：contract_id+version；1062 → 重签冲突）。
		InsertTx(ctx context.Context, session sqlx.Session, data *ContractFile) error
		// ListByContract 版本列表（新→旧）。
		ListByContract(ctx context.Context, tenantId, contractId int64) ([]*ContractFile, error)
		// CountActive 生效前校验（≥1 份 ACTIVE）。
		CountActive(ctx context.Context, tenantId, contractId int64) (int64, error)
		// ReplaceOldTx 旧版本置 REPLACED（重签）。
		ReplaceOldTx(ctx context.Context, session sqlx.Session, tenantId, contractId, version int64) error
		// MaxVersion 当前最大版本号。
		MaxVersion(ctx context.Context, tenantId, contractId int64) (int64, error)
	}

	customContractFileModel struct {
		*defaultContractFileModel
		conn sqlx.SqlConn
	}
)

// NewContractFileModel returns a model for the database table.
func NewContractFileModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ContractFileModel {
	return &customContractFileModel{
		defaultContractFileModel: newContractFileModel(conn, c, opts...),
		conn:                     conn,
	}
}

func (m *customContractFileModel) InsertTx(ctx context.Context, session sqlx.Session, data *ContractFile) error {
	query := "insert into `contract_file` (`file_rec_id`, `contract_id`, `file_id`, `file_name`, `version`, `sign_party_name`, `sign_party_type`, `status`, `uploaded_by`, `uploader_name`, `remark`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.FileRecId, data.ContractId, data.FileId, data.FileName,
		data.Version, data.SignPartyName, data.SignPartyType, data.Status, data.UploadedBy,
		data.UploaderName, data.Remark, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customContractFileModel) ListByContract(ctx context.Context, tenantId, contractId int64) ([]*ContractFile, error) {
	var list []*ContractFile
	query := "select " + contractFileRows + " from `contract_file` where `contract_id` = ? and `tenant_id` = ? and `deleted_at` is null order by `version` desc"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, contractId, tenantId); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customContractFileModel) CountActive(ctx context.Context, tenantId, contractId int64) (int64, error) {
	var cnt int64
	query := "select count(*) from `contract_file` where `contract_id` = ? and `tenant_id` = ? and `status` = 'ACTIVE' and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &cnt, query, contractId, tenantId); err != nil {
		return 0, err
	}
	return cnt, nil
}

func (m *customContractFileModel) ReplaceOldTx(ctx context.Context, session sqlx.Session, tenantId, contractId, version int64) error {
	query := "update `contract_file` set `status` = 'REPLACED' where `contract_id` = ? and `tenant_id` = ? and `version` < ? and `status` = 'ACTIVE'"
	_, err := session.ExecCtx(ctx, query, contractId, tenantId, version)
	return err
}

func (m *customContractFileModel) MaxVersion(ctx context.Context, tenantId, contractId int64) (int64, error) {
	var max sql.NullInt64
	query := "select max(`version`) from `contract_file` where `contract_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &max, query, contractId, tenantId); err != nil {
		return 0, err
	}
	return max.Int64, nil
}

// —— warranty ——

var _ WarrantyModel = (*customWarrantyModel)(nil)

type (
	// WarrantyModel 质保模型（STATION/DEVICE 双层 + start_rule 快照，FR-CTR-005）。
	WarrantyModel interface {
		warrantyModel
		// InsertTx 事务内建质保计划（PENDING 待起算）。
		InsertTx(ctx context.Context, session sqlx.Session, data *Warranty) error
		// FindOneByNo 租户过滤按质保号取。
		FindOneByNo(ctx context.Context, tenantId int64, warrantyNo string) (*Warranty, error)
		// FindOneScoped 租户过滤按 ID 取。
		FindOneScoped(ctx context.Context, tenantId, warrantyId int64) (*Warranty, error)
		// FindByTarget 双层查询（target_id 或 target_key）。
		FindByTarget(ctx context.Context, tenantId int64, targetType string, targetId int64, targetKey string) ([]*Warranty, error)
		// StartTx 起算（事件驱动：PENDING → ACTIVE + start/end；affected=0 即重复事件/已起算）。
		StartTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64, startAt, endAt time.Time) error
		// MarkRefundedTx 延保退款联动。
		MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64) error
		// ListPage 质保列表。
		ListPage(ctx context.Context, tenantId int64, status, level string, page, size int64) ([]*Warranty, int64, error)
		// MarkExpiredTx 到期扫描（ACTIVE 且 end_at < now → EXPIRED）。
		MarkExpiredTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64) error
		// FindExpired 到期扫描集。
		FindExpired(ctx context.Context, now time.Time, limit int) ([]*Warranty, error)
	}

	customWarrantyModel struct {
		*defaultWarrantyModel
		conn sqlx.SqlConn
	}
)

// NewWarrantyModel returns a model for the database table.
func NewWarrantyModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) WarrantyModel {
	return &customWarrantyModel{
		defaultWarrantyModel: newWarrantyModel(conn, c, opts...),
		conn:                 conn,
	}
}

func (m *customWarrantyModel) InsertTx(ctx context.Context, session sqlx.Session, data *Warranty) error {
	query := "insert into `warranty` (`warranty_id`, `warranty_no`, `level`, `target_type`, `target_id`, `target_key`, `status`, `months`, `start_rule`, `source_type`, `source_no`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.WarrantyId, data.WarrantyNo, data.Level, data.TargetType,
		data.TargetId, data.TargetKey, data.Status, data.Months, data.StartRule, data.SourceType,
		data.SourceNo, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customWarrantyModel) FindOneByNo(ctx context.Context, tenantId int64, warrantyNo string) (*Warranty, error) {
	var res Warranty
	query := "select " + warrantyRows + " from `warranty` where `warranty_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, warrantyNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customWarrantyModel) FindOneScoped(ctx context.Context, tenantId, warrantyId int64) (*Warranty, error) {
	var res Warranty
	query := "select " + warrantyRows + " from `warranty` where `warranty_id` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, warrantyId, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customWarrantyModel) FindByTarget(ctx context.Context, tenantId int64, targetType string, targetId int64, targetKey string) ([]*Warranty, error) {
	where := "`target_type` = ? and `tenant_id` = ? and `deleted_at` is null"
	args := []any{targetType, tenantId}
	if targetId > 0 {
		where += " and `target_id` = ?"
		args = append(args, targetId)
	}
	if targetKey != "" {
		where += " and `target_key` = ?"
		args = append(args, targetKey)
	}
	var list []*Warranty
	query := "select " + warrantyRows + " from `warranty` where " + where + " order by `warranty_id` desc"
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	return list, nil
}

func (m *customWarrantyModel) StartTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64, startAt, endAt time.Time) error {
	query := "update `warranty` set `status` = 'ACTIVE', `start_at` = ?, `end_at` = ? where `warranty_id` = ? and `tenant_id` = ? and `status` = 'PENDING'"
	res, err := session.ExecCtx(ctx, query, startAt, endAt, warrantyId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customWarrantyModel) MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64) error {
	query := "update `warranty` set `status` = 'REFUNDED' where `warranty_id` = ? and `tenant_id` = ?"
	_, err := session.ExecCtx(ctx, query, warrantyId, tenantId)
	return err
}

func (m *customWarrantyModel) ListPage(ctx context.Context, tenantId int64, status, level string, page, size int64) ([]*Warranty, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if status != "" {
		where += " and `status` = ?"
		args = append(args, status)
	}
	if level != "" {
		where += " and `level` = ?"
		args = append(args, level)
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, "select count(*) from `warranty` where "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Warranty
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, "select "+warrantyRows+" from `warranty` where "+where+" order by `warranty_id` desc limit ? offset ?", args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customWarrantyModel) MarkExpiredTx(ctx context.Context, session sqlx.Session, tenantId, warrantyId int64) error {
	query := "update `warranty` set `status` = 'EXPIRED' where `warranty_id` = ? and `tenant_id` = ? and `status` = 'ACTIVE'"
	_, err := session.ExecCtx(ctx, query, warrantyId, tenantId)
	return err
}

func (m *customWarrantyModel) FindExpired(ctx context.Context, now time.Time, limit int) ([]*Warranty, error) {
	var list []*Warranty
	query := "select " + warrantyRows + " from `warranty` where `status` = 'ACTIVE' and `end_at` is not null and `end_at` <= ? and `deleted_at` is null limit ?"
	if err := m.conn.QueryRowCtx(ctx, &list, query, now, limit); err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return list, nil
}

// —— warranty_extension ——

var _ WarrantyExtensionModel = (*customWarrantyExtensionModel)(nil)

type (
	// WarrantyExtensionModel 延保模型（销售/衔接/转移/退款，FR-CTR-006）。
	WarrantyExtensionModel interface {
		warrantyExtensionModel
		// InsertTx 延保落库。
		InsertTx(ctx context.Context, session sqlx.Session, data *WarrantyExtension) error
		// FindOneByNo 租户过滤按延保号取。
		FindOneByNo(ctx context.Context, tenantId int64, extensionNo string) (*WarrantyExtension, error)
		// CASStatusTx 状态机（ACTIVE/TRANSFERRED/REFUNDED）。
		CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, from []string, to string) error
		// MarkTransferredTx 转移回写。
		MarkTransferredTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, targetType string, targetId int64, targetKey string) error
		// MarkRefundedTx 退款回写。
		MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, refundNo string) error
	}

	customWarrantyExtensionModel struct {
		*defaultWarrantyExtensionModel
		conn sqlx.SqlConn
	}
)

// NewWarrantyExtensionModel returns a model for the database table.
func NewWarrantyExtensionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) WarrantyExtensionModel {
	return &customWarrantyExtensionModel{
		defaultWarrantyExtensionModel: newWarrantyExtensionModel(conn, c, opts...),
		conn:                          conn,
	}
}

func (m *customWarrantyExtensionModel) InsertTx(ctx context.Context, session sqlx.Session, data *WarrantyExtension) error {
	query := "insert into `warranty_extension` (`extension_id`, `extension_no`, `base_warranty_id`, `months`, `order_no`, `amount`, `status`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, data.ExtensionId, data.ExtensionNo, data.BaseWarrantyId,
		data.Months, data.OrderNo, data.Amount, data.Status, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customWarrantyExtensionModel) FindOneByNo(ctx context.Context, tenantId int64, extensionNo string) (*WarrantyExtension, error) {
	var res WarrantyExtension
	query := "select " + warrantyExtensionRows + " from `warranty_extension` where `extension_no` = ? and `tenant_id` = ? and `deleted_at` is null"
	if err := m.QueryRowNoCacheCtx(ctx, &res, query, extensionNo, tenantId); err != nil {
		return nil, err
	}
	return &res, nil
}

func (m *customWarrantyExtensionModel) CASStatusTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, from []string, to string) error {
	placeholders := ""
	args := []any{to, extensionId, tenantId}
	for i, s := range from {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, s)
	}
	query := "update `warranty_extension` set `status` = ? where `extension_id` = ? and `tenant_id` = ? and `status` in (" + placeholders + ")"
	res, err := session.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customWarrantyExtensionModel) MarkTransferredTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, targetType string, targetId int64, targetKey string) error {
	query := "update `warranty_extension` set `status` = 'TRANSFERRED', `to_target_type` = ?, `to_target_id` = ?, `to_target_key` = ? where `extension_id` = ? and `tenant_id` = ? and `status` = 'ACTIVE'"
	res, err := session.ExecCtx(ctx, query, targetType, targetId, targetKey, extensionId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}

func (m *customWarrantyExtensionModel) MarkRefundedTx(ctx context.Context, session sqlx.Session, tenantId, extensionId int64, refundNo string) error {
	query := "update `warranty_extension` set `status` = 'REFUNDED', `refund_no` = ? where `extension_id` = ? and `tenant_id` = ? and `status` = 'ACTIVE'"
	res, err := session.ExecCtx(ctx, query, refundNo, extensionId, tenantId)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrStatusConflict
	}
	return nil
}
