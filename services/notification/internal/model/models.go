// notification_db 各表 custom 覆写（ADR-08）：租户过滤 + 显式列更新 + NoCache 查询。
// 本文件聚合 5 张表的 custom 实现（表结构简单，单文件便于对照维护）。

package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// —— message ——

var _ MessageModel = (*customMessageModel)(nil)

type (
	// MessageModel 站内信模型。
	MessageModel interface {
		// Insert 新建站内信（生成方法；data.TenantId 必填）。
		Insert(ctx context.Context, data *Message) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, messageId int64) (*Message, error)
		// FindPageByUser 收件箱（仅未读过滤；新→旧）。
		FindPageByUser(ctx context.Context, tenantId, userId int64, onlyUnread bool, page, size int64) ([]*Message, int64, error)
		// CountUnread 未读数。
		CountUnread(ctx context.Context, tenantId, userId int64) (int64, error)
		// MarkRead 标记已读（本人校验在 SQL 条件内）。
		MarkRead(ctx context.Context, tenantId, messageId, userId int64) error
		// CountByTemplate24h 频控：同用户同模板 24h 内条数（模板码记在 biz_type 前缀,见 Deliver 内核）。
		CountByBiz24h(ctx context.Context, tenantId, userId int64, bizKey string) (int64, error)
	}

	customMessageModel struct {
		*defaultMessageModel
	}
)

func NewMessageModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) MessageModel {
	return &customMessageModel{defaultMessageModel: newMessageModel(conn, c, opts...)}
}

func (m *customMessageModel) FindOne(ctx context.Context, tenantId, messageId int64) (*Message, error) {
	var resp Message
	query := fmt.Sprintf("select %s from %s where `message_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", messageRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, messageId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMessageModel) FindPageByUser(ctx context.Context, tenantId, userId int64, onlyUnread bool, page, size int64) ([]*Message, int64, error) {
	where := "`tenant_id` = ? and `user_id` = ? and `deleted_at` is null"
	args := []any{tenantId, userId}
	if onlyUnread {
		where += " and `is_read` = 0"
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, fmt.Sprintf("select count(*) from %s where %s", m.table, where), args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*Message
	query := fmt.Sprintf("select %s from %s where %s order by `message_id` desc limit ? offset ?", messageRows, m.table, where)
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customMessageModel) CountUnread(ctx context.Context, tenantId, userId int64) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `user_id` = ? and `is_read` = 0 and `deleted_at` is null", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId, userId)
	return n, err
}

func (m *customMessageModel) MarkRead(ctx context.Context, tenantId, messageId, userId int64) error {
	query := fmt.Sprintf("update %s set `is_read` = 1, `updated_at` = ? where `message_id` = ? and `user_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, time.Now(), messageId, userId, tenantId)
	return err
}

func (m *customMessageModel) CountByBiz24h(ctx context.Context, tenantId, userId int64, bizKey string) (int64, error) {
	var n int64
	query := fmt.Sprintf("select count(*) from %s where `tenant_id` = ? and `user_id` = ? and `biz_type` = ? and `created_at` >= ? and `deleted_at` is null", m.table)
	err := m.QueryRowNoCacheCtx(ctx, &n, query, tenantId, userId, bizKey, time.Now().Add(-24*time.Hour))
	return n, err
}

// —— notify_template ——

var _ NotifyTemplateModel = (*customNotifyTemplateModel)(nil)

type (
	// NotifyTemplateModel 通知模板模型。
	NotifyTemplateModel interface {
		// Insert 新建模板（生成方法）。
		Insert(ctx context.Context, data *NotifyTemplate) (sql.Result, error)
		// FindOne 按 ID 取（租户过滤）。
		FindOne(ctx context.Context, tenantId, templateId int64) (*NotifyTemplate, error)
		// FindByCode 按模板码取（Deliver 内核渲染用）。
		FindByCode(ctx context.Context, tenantId int64, code string) (*NotifyTemplate, error)
		// FindPage 模板列表（keyword 模糊 code）。
		FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*NotifyTemplate, int64, error)
		// UpdateColumns 显式列更新。
		UpdateColumns(ctx context.Context, tenantId, templateId int64, titleTpl, contentTpl, channel string, status int64, updatedBy int64) error
	}

	customNotifyTemplateModel struct {
		*defaultNotifyTemplateModel
	}
)

func NewNotifyTemplateModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) NotifyTemplateModel {
	return &customNotifyTemplateModel{defaultNotifyTemplateModel: newNotifyTemplateModel(conn, c, opts...)}
}

func (m *customNotifyTemplateModel) FindOne(ctx context.Context, tenantId, templateId int64) (*NotifyTemplate, error) {
	var resp NotifyTemplate
	query := fmt.Sprintf("select %s from %s where `template_id` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", notifyTemplateRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, templateId, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customNotifyTemplateModel) FindByCode(ctx context.Context, tenantId int64, code string) (*NotifyTemplate, error) {
	var resp NotifyTemplate
	query := fmt.Sprintf("select %s from %s where `code` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", notifyTemplateRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, code, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customNotifyTemplateModel) FindPage(ctx context.Context, tenantId int64, keyword string, page, size int64) ([]*NotifyTemplate, int64, error) {
	where := "`tenant_id` = ? and `deleted_at` is null"
	args := []any{tenantId}
	if keyword != "" {
		where += " and `code` like ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if err := m.QueryRowNoCacheCtx(ctx, &total, fmt.Sprintf("select count(*) from %s where %s", m.table, where), args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}
	var list []*NotifyTemplate
	query := fmt.Sprintf("select %s from %s where %s order by `template_id` desc limit ? offset ?", notifyTemplateRows, m.table, where)
	args = append(args, size, (page-1)*size)
	if err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *customNotifyTemplateModel) UpdateColumns(ctx context.Context, tenantId, templateId int64, titleTpl, contentTpl, channel string, status int64, updatedBy int64) error {
	query := fmt.Sprintf("update %s set `title_template` = ?, `content_template` = ?, `channel` = ?, `status` = ?, `updated_by` = ?, `updated_at` = ? where `template_id` = ? and `tenant_id` = ? and `deleted_at` is null", m.table)
	_, err := m.ExecNoCacheCtx(ctx, query, titleTpl, contentTpl, channel, status, ToNullInt64(updatedBy), time.Now(), templateId, tenantId)
	return err
}

// —— outbound_log ——

var _ OutboundLogModel = (*customOutboundLogModel)(nil)

type (
	// OutboundLogModel 外发流水模型（append-only）。
	OutboundLogModel interface {
		// Insert 追加外发流水（生成方法）。
		Insert(ctx context.Context, data *OutboundLog) (sql.Result, error)
	}

	customOutboundLogModel struct {
		*defaultOutboundLogModel
	}
)

func NewOutboundLogModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OutboundLogModel {
	return &customOutboundLogModel{defaultOutboundLogModel: newOutboundLogModel(conn, c, opts...)}
}

// —— user_notify_setting ——

var _ UserNotifySettingModel = (*customUserNotifySettingModel)(nil)

type (
	// UserNotifySettingModel 用户通知设置模型。
	UserNotifySettingModel interface {
		// Upsert 幂等写入/更新（uk_setting_user_code）。
		Upsert(ctx context.Context, data *UserNotifySetting) error
		// FindByUser 查用户设置（templateCode 空 = 全部）。
		FindByUser(ctx context.Context, tenantId, userId int64, templateCode string) ([]*UserNotifySetting, error)
	}

	customUserNotifySettingModel struct {
		*defaultUserNotifySettingModel
	}
)

func NewUserNotifySettingModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserNotifySettingModel {
	return &customUserNotifySettingModel{defaultUserNotifySettingModel: newUserNotifySettingModel(conn, c, opts...)}
}

func (m *customUserNotifySettingModel) Upsert(ctx context.Context, data *UserNotifySetting) error {
	query := "insert into `user_notify_setting` (`setting_id`, `user_id`, `template_code`, `enabled`, `quiet_hours`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?) on duplicate key update `enabled` = values(`enabled`), `quiet_hours` = values(`quiet_hours`), `updated_by` = values(`updated_by`), `updated_at` = current_timestamp"
	_, err := m.ExecNoCacheCtx(ctx, query, data.SettingId, data.UserId, data.TemplateCode, data.Enabled,
		data.QuietHours, data.TenantId, data.CreatedBy, data.UpdatedBy)
	return err
}

func (m *customUserNotifySettingModel) FindByUser(ctx context.Context, tenantId, userId int64, templateCode string) ([]*UserNotifySetting, error) {
	where := "`tenant_id` = ? and `user_id` = ? and `deleted_at` is null"
	args := []any{tenantId, userId}
	if templateCode != "" {
		where += " and `template_code` = ?"
		args = append(args, templateCode)
	}
	var list []*UserNotifySetting
	query := fmt.Sprintf("select %s from %s where %s order by `setting_id` desc", userNotifySettingRows, m.table, where)
	err := m.QueryRowsNoCacheCtx(ctx, &list, query, args...)
	if err != nil && err != ErrNotFound {
		return nil, err
	}
	return list, nil
}

// —— notify_channel ——

var _ NotifyChannelModel = (*customNotifyChannelModel)(nil)

type (
	// NotifyChannelModel 渠道配置模型。
	NotifyChannelModel interface {
		// Insert 新建渠道（生成方法）。
		Insert(ctx context.Context, data *NotifyChannel) (sql.Result, error)
		// FindByCode 按渠道码取。
		FindByCode(ctx context.Context, tenantId int64, code string) (*NotifyChannel, error)
	}

	customNotifyChannelModel struct {
		*defaultNotifyChannelModel
	}
)

func NewNotifyChannelModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) NotifyChannelModel {
	return &customNotifyChannelModel{defaultNotifyChannelModel: newNotifyChannelModel(conn, c, opts...)}
}

func (m *customNotifyChannelModel) FindByCode(ctx context.Context, tenantId int64, code string) (*NotifyChannel, error) {
	var resp NotifyChannel
	query := fmt.Sprintf("select %s from %s where `code` = ? and `tenant_id` = ? and `deleted_at` is null limit 1", notifyChannelRows, m.table)
	err := m.QueryRowNoCacheCtx(ctx, &resp, query, code, tenantId)
	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// —— 通用空值助手（导出供 logic/consumer 层复用）——

// ToNullString 字符串空值转换。
func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// ToNullInt64 int64 空值转换。
func ToNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}
