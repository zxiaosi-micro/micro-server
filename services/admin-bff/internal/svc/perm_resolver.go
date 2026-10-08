package svc

import "net/http"

// permResolver 路由 → 权限码解析（authz RBAC ④ 段消费）。
// 返回 ok=false 的路由仅要求登录（/auth/me /auth/menus 等）。
// 与 menu 表 perm_code 同源（000003_menu_seed），变更权限码须两处同步（S7 演进为启动期从 identity 拉取）。
func permResolver(r *http.Request) (string, bool) {
	code, ok := routePerms[r.Method+" "+routeKey(r)]
	return code, ok
}

// routeKey 剥去 /api/v1 前缀并把 /users/123 折叠为 /users/:id，与 routePerms 对齐。
func routeKey(r *http.Request) string {
	segs := splitPath(r.URL.Path)
	// 剥 /api/v1（goctl prefix；path 规范由路由保证）
	if len(segs) >= 2 && segs[0] == "api" && segs[1] == "v1" {
		segs = segs[2:]
	}
	segments := segs
	out := make([]string, 0, len(segments))
	for i, seg := range segments {
		// 最后一段是数字 ID 且前缀是已知资源 → :id
		if isNumericID(seg) && i > 0 {
			out = append(out, ":id")
			continue
		}
		out = append(out, seg)
	}
	return joinPath(out)
}

var routePerms = map[string]string{
	// 用户
	"GET /users":              "system:user:list",
	"GET /users/:id":          "system:user:list",
	"POST /users":             "system:user:create",
	"PUT /users/:id":          "system:user:update",
	"PUT /users/:id/status":   "system:user:status",
	"PUT /users/:id/password": "system:user:reset-pwd",
	"DELETE /users/:id":       "system:user:delete",
	// 角色
	"GET /roles":        "system:role:list",
	"GET /roles/:id":    "system:role:list",
	"POST /roles":       "system:role:create",
	"PUT /roles/:id":    "system:role:update",
	"DELETE /roles/:id": "system:role:delete",
	// 组织
	"GET /orgs":        "system:org:list",
	"POST /orgs":       "system:org:create",
	"PUT /orgs/:id":    "system:org:update",
	"DELETE /orgs/:id": "system:org:delete",
	// 菜单
	"GET /menus":        "system:menu:list",
	"POST /menus":       "system:menu:create",
	"PUT /menus/:id":    "system:menu:update",
	"DELETE /menus/:id": "system:menu:delete",
	// 租户
	"GET /tenants":           "system:tenant:list",
	"POST /tenants":          "system:tenant:create",
	"PUT /tenants/:id":       "system:tenant:update",
	"PUT /tenants/:id/quota": "system:tenant:quota",
	"DELETE /tenants/:id":    "system:tenant:update",
	// 会话
	"GET /sessions":       "system:session:list",
	"POST /sessions/kick": "system:session:kick",

	// —— S4 业务域（S4-05；与 000004_s4_menu_seed perm_code 同源）——
	// 参与方
	"GET /parties":                "party:party:list",
	"GET /parties/:id":            "party:party:list",
	"POST /parties":               "party:party:create",
	"PUT /parties/:id":            "party:party:update",
	"GET /parties/:id/contacts":   "party:party:list",
	"POST /parties/:id/contacts":  "party:contact:create",
	"GET /parties/:id/crm":        "party:party:list",
	"POST /parties/:id/crm":       "party:crm:create",
	"GET /parties/:id/staff":      "party:party:list",
	"POST /staff":                 "party:staff:create",
	"GET /parties/:id/dealer-ext": "party:party:list",
	"PUT /parties/:id/dealer-ext": "party:dealer:update",
	// 商机
	"GET /opportunities":           "party:opportunity:list",
	"POST /opportunities":          "party:opportunity:create",
	"PUT /opportunities/:id/stage": "party:opportunity:update",
	// 商品
	"GET /products":                 "catalog:product:list",
	"POST /products":                "catalog:product:create",
	"GET /skus":                     "catalog:sku:list",
	"POST /skus":                    "catalog:sku:create",
	"GET /prices":                   "catalog:price:list",
	"POST /skus/:id/prices":         "catalog:price:set",
	"PUT /skus/:id/warranty-policy": "catalog:warranty:update",
	"GET /station-products":         "catalog:station:list",
	"GET /station-products/:id":     "catalog:station:list",
	"POST /station-products":        "catalog:station:create",
	// 库存
	"GET /warehouses":              "inventory:warehouse:list",
	"POST /warehouses":             "inventory:warehouse:create",
	"GET /inventory":               "inventory:inventory:list",
	"POST /inventory/stock-in":     "inventory:stock:in",
	"POST /inventory/reserve":      "inventory:stock:reserve",
	"POST /inventory/release":      "inventory:stock:release",
	"POST /inventory/deduct":       "inventory:stock:deduct",
	"POST /inventory/spare-out":    "inventory:stock:spare-out",
	"POST /inventory/spare-return": "inventory:stock:spare-return",
	"GET /inventory/records":       "inventory:record:list",
	"GET /stocktakes":              "inventory:stocktake:list",
	"GET /stocktakes/:id":          "inventory:stocktake:list",
	"POST /stocktakes":             "inventory:stocktake:create",
	"POST /stocktakes/:id/submit":  "inventory:stocktake:submit",
	"POST /stocktakes/:id/approve": "inventory:stocktake:approve",
	// 消息
	"GET /messages":              "notification:message:list",
	"GET /messages/unread-count": "notification:message:list",
	"POST /messages/:id/read":    "notification:message:read",
	"GET /notify-templates":      "notification:template:list",
	"POST /notify-templates":     "notification:template:update",
	"GET /notify-settings":       "notification:setting:list",
	"POST /notify-settings":      "notification:setting:update",
	// 审计
	"GET /audit-logs": "audit:log:list",
	"GET /cmd-logs":   "audit:cmd:list",
}

func splitPath(p string) []string {
	var out []string
	for _, seg := range segments(p) {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func segments(p string) []string {
	var out []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			out = append(out, p[start:i])
			start = i + 1
		}
	}
	out = append(out, p[start:])
	return out
}

func joinPath(parts []string) string {
	out := ""
	for _, p := range parts {
		out += "/" + p
	}
	return out
}

func isNumericID(s string) bool {
	if len(s) == 0 || len(s) > 20 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
