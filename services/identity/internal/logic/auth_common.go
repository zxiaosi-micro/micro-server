// 认证主链公共逻辑：token 签发、auth_cache 组装、锁定映射、类型工具。

package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
	"github.com/zxiaosi-micro/micro-common/sessionx"

	"micro-server/services/identity/internal/model"
	"micro-server/services/identity/internal/svc"
)

// issueTokens 签发 token 对：sessionx.Create（互斥踢旧）→ IssueRefresh → Sign access。
// 返回 access/refresh 与被踢会话（调用方审计留痕）。
func issueTokens(ctx context.Context, sc *svc.ServiceContext, uid int64, client string) (
	access, refresh string, accessExp, refreshExp int64, kicked []string, err error) {
	sess, kicked, err := sc.Sessions.Create(ctx, uid, client)
	if err != nil {
		return "", "", 0, 0, nil, err
	}
	rtid, err := sc.Sessions.IssueRefresh(ctx, uid, sess.SID)
	if err != nil {
		return "", "", 0, 0, nil, err
	}
	claims := jwtauth.Claims{UID: uid, SID: sess.SID, Client: client, Level: 1}
	access, err = sc.Signer.Sign(claims, time.Duration(sc.AccessTTL)*time.Second)
	if err != nil {
		return "", "", 0, 0, nil, err
	}
	return access, rtid, sc.AccessTTL, sc.RefreshTTL, kicked, nil
}

// ensureAuthCache 组装并写入 auth_cache（登录成功后必调；角色/菜单变更另行失效）。
func ensureAuthCache(ctx context.Context, sc *svc.ServiceContext, tenantID, uid int64) error {
	roles, perms, dataScope, err := loadRBAC(ctx, sc.Conn, sc.Models, tenantID, uid)
	if err != nil {
		return err
	}
	return sc.Sessions.PutAuthCache(ctx, uid, &sessionx.AuthSnapshot{
		Roles:     roles,
		Perms:     perms,
		DataScope: dataScope,
		TenantID:  tenantID,
	}, sc.SessionTTL)
}

// loadRBAC 读角色码 + 权限码集合（user_role → role → role_menu → menu.perm_code）。
// 直接走 sc.Models（避免 helper 反复构造模型）。
func loadRBAC(ctx context.Context, conn sqlx.SqlConn, m *svc.Models, tenantID, uid int64) (roles, perms []string, dataScope string, err error) {
	roleIds, err := m.UserRole.FindRoleIdsByUser(ctx, tenantID, uid)
	if err != nil {
		return nil, nil, "", err
	}
	if len(roleIds) == 0 {
		return []string{}, []string{}, "{}", nil
	}
	permSet := map[string]struct{}{}
	dataScope = "{}"
	for _, rid := range roleIds {
		role, err := m.Role.FindOne(ctx, tenantID, rid)
		if err != nil {
			if err == model.ErrNotFound {
				continue // 脏绑定（角色已删）
			}
			return nil, nil, "", err
		}
		roles = append(roles, role.Code)
		if role.DataScope.Valid && role.DataScope.String != "" {
			// 多角色数据域合并：ALL > ORG > SELF（当前种子单角色；多角色并集 S4 演进）
			dataScope = mergeDataScope(dataScope, role.DataScope.String)
		}
		menuIds, err := m.RoleMenu.FindMenuIdsByRole(ctx, tenantID, rid)
		if err != nil {
			return nil, nil, "", err
		}
		for _, mid := range menuIds {
			menu, err := m.Menu.FindOne(ctx, tenantID, mid)
			if err != nil {
				if err == model.ErrNotFound {
					continue
				}
				return nil, nil, "", err
			}
			if menu.PermCode.Valid && menu.PermCode.String != "" {
				permSet[menu.PermCode.String] = struct{}{}
			}
		}
	}
	perms = make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}
	if roles == nil {
		roles = []string{}
	}
	return roles, perms, dataScope, nil
}

type dataScopeJSON struct {
	Type   string  `json:"type"`
	OrgIDs []int64 `json:"org_ids"`
}

var dataScopeRank = map[string]int{"ALL": 3, "ORG": 2, "SELF": 1}

// mergeDataScope 多角色数据域取最宽（ALL > ORG > SELF）。
func mergeDataScope(cur, next string) string {
	var c, n dataScopeJSON
	if err := json.Unmarshal([]byte(cur), &c); err != nil {
		return next
	}
	if err := json.Unmarshal([]byte(next), &n); err != nil {
		return cur
	}
	if dataScopeRank[n.Type] > dataScopeRank[c.Type] {
		return next
	}
	return cur
}

// checkClientType 校验请求端是否在该账号可登录端集合内（user.types）。
func checkClientType(typesJSON, client string) bool {
	for _, t := range decodeTypes(typesJSON) {
		if t == client {
			return true
		}
	}
	return false
}

func decodeTypes(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// loginIdent 锁定计数键：mobile_hash（uid 未登录前不可得，sessionx 口径）。
func loginIdent(mobileHash string) string { return mobileHash }

// wrapLocked 把 sessionx 锁定状态映射为 10407（msg 携带剩余秒数，前端倒计时）。
func wrapLocked(state sessionx.LockedState) error {
	msg := "失败次数过多,账号已锁定"
	if secs := int64(state.Remaining / time.Second); secs > 0 {
		msg = "失败次数过多,账号已锁定," + itoa(secs) + " 秒后自动解锁"
	}
	return errcode.ErrLoginLocked.WithMsg(msg)
}

// opUID 从 ctx 取操作人（BFF 经 metadata 桥注入；0 = 系统调用）。
func opUID(ctx context.Context) int64 { return ctxkit.UID(ctx) }
