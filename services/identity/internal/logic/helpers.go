// 内部公共助手：审计事件、索引哈希、上下文取值。

package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/eventbus"

	"micro-server/services/identity/internal/svc"
)

// defaultTenantID 影子账号落 default 租户（tenant_code UK 定位；该租户由 tools/seed 种入）。
// 影子账号是微信首登的兜底流程；正式业务租户账号由管理端建号，不走此路径。
func defaultTenantID(sc *svc.ServiceContext) (int64, error) {
	t, err := sc.Models.Tenant.FindByCode(context.Background(), "default")
	if err != nil {
		return 0, errcode.Internal.WithMsg("default 租户未初始化").WithCause(err)
	}
	return t.TenantId, nil
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// hashIndex HMAC-SHA256 索引哈希（mobile_hash/email_hash；与 tools/seed 口径一致）。
func (l *LoginByPasswordLogic) hashIndex(plain string) string {
	return hmacHex(l.svcCtx.HashKey, plain)
}

func (l *CreateUserLogic) hashIndex(plain string) string { return hmacHex(l.svcCtx.HashKey, plain) }

func (l *UpdateUserLogic) hashIndex(plain string) string { return hmacHex(l.svcCtx.HashKey, plain) }

func hmacHex(key []byte, plain string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(plain))
	return hex.EncodeToString(mac.Sum(nil))
}

// auditLogin 写 auth.login 审计事件（outbox；Relay 投递 Kafka 属 S5）。
// outbox 强约束与业务同事务——此处登录路径无 MySQL 变更，走独立连接提交
// （eventbus.Emit 评审说明：登录是 Redis 侧原子操作，无本地事务可挂）。
func auditLogin(ctx context.Context, conn sqlx.SqlConn, uid, tenantID int64, typ string) {
	err := eventbus.Emit(ctx, conn, eventbus.EmitInput{
		Topic:    eventbus.TopicAuditEvent,
		Type:     typ,
		Key:      itoa(uid),
		TenantID: tenantID,
		Payload: map[string]any{
			"uid":        uid,
			"tenant_id":  tenantID,
			"client":     ctxkit.Client(ctx),
			"login_type": typ,
		},
	})
	if err != nil {
		// 审计写失败不阻断登录（Redis 会话已生效），留 ERROR 告警日志人工对账
		logx.WithContext(ctx).Errorf("audit %s outbox 写入失败 uid=%d: %v", typ, uid, err)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
