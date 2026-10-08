package logic

import (
	"context"

	"micro-server/services/party/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// insertContact 事务内显式列 INSERT（生成方法不接 session，02 §6.2 样例口径）。
func insertContact(ctx context.Context, session sqlx.Session, sc *svc.ServiceContext, tid, op, partyId int64,
	name, mobileCipher, mobileHash, position string, isDefault int64, notifyPref string, cid int64) error {
	query := "insert into `contact` (`contact_id`, `party_id`, `name`, `mobile`, `mobile_hash`, `position`, `is_default`, `notify_pref`, `tenant_id`, `created_by`, `updated_by`) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	_, err := session.ExecCtx(ctx, query, cid, partyId, name, mobileCipher, mobileHash,
		position, isDefault, notifyPref, tid, toNullInt64(op), toNullInt64(op))
	return err
}
