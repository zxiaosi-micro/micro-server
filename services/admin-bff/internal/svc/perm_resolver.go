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
		// 数字 ID → :id；业务单号（ORD/RET/SHP/PAY/RFD/INV/CTR/CLM/EXT 前缀）→ :no
		if i > 0 && (isNumericID(seg) || isBizNo(seg)) {
			out = append(out, ":no")
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

	// —— S5 交易域（S5-02~04；与 000005_s5_menu_seed perm_code 同源）——
	// 订单
	"GET /orders":                        "trade:order:list",
	"GET /orders/:no":                    "trade:order:list",
	"GET /orders/:no/saga":               "trade:order:list",
	"POST /orders":                       "trade:order:create",
	"POST /orders/:no/pay":               "trade:order:pay",
	"POST /orders/:no/cancel":            "trade:order:cancel",
	"POST /sagas/:no/retry":              "trade:order:retry",
	"GET /sales-summary":                 "trade:order:list",
	"POST /return-orders":                "trade:return:create",
	"POST /return-orders/:no/approve":    "trade:return:approve",
	"POST /shipments":                    "trade:shipment:create",
	"POST /shipments/:no/traces":         "trade:shipment:trace",
	"POST /shipments/:no/confirm-signed": "trade:shipment:sign",
	// 支付/退款/发票/对账
	"GET /payments":                     "finance:payment:list",
	"POST /payments/:no/confirm":        "finance:payment:confirm",
	"POST /payments/:no/approve":        "finance:payment:approve",
	"POST /payments/:no/settle":         "finance:payment:settle",
	"GET /refunds":                      "finance:refund:list",
	"POST /refunds":                     "finance:refund:create",
	"GET /invoices":                     "finance:invoice:list",
	"POST /invoices":                    "finance:invoice:issue",
	"POST /invoices/:no/reverse":        "finance:invoice:reverse",
	"GET /reconcile-tasks":              "finance:reconcile:list",
	"POST /reconcile-tasks/:no/resolve": "finance:reconcile:resolve",
	// 合同/质保/SLA/索赔/延保
	"GET /contracts":                         "contract:contract:list",
	"GET /contracts/:no":                     "contract:contract:list",
	"POST /contracts/:no/files":              "contract:contract:archive",
	"POST /contracts/:no/archive":            "contract:contract:archive",
	"POST /contracts/:no/sla":                "contract:sla:bind",
	"GET /warranties":                        "contract:warranty:list",
	"GET /warranty-by-target":                "contract:warranty:list",
	"GET /sla-strategies":                    "contract:sla:list",
	"POST /sla-strategies":                   "contract:sla:create",
	"GET /claims":                            "contract:claim:list",
	"POST /claims":                           "contract:claim:create",
	"POST /claims/:no/approve":               "contract:claim:approve",
	"POST /claims/:no/settle":                "contract:claim:settle",
	"POST /warranty-extensions":              "contract:extension:sell",
	"POST /warranty-extensions/:no/transfer": "contract:extension:transfer",
	"POST /warranty-extensions/:no/refund":   "contract:extension:refund",

	// S6-01 设备域（asset:device/ota）
	"POST /devices/import":         "asset:device:create",
	"GET /devices":                 "asset:device:list",
	"GET /devices/:no":             "asset:device:list",
	"POST /devices/:no/transition": "asset:device:transition",
	"POST /devices/:no/activate":   "asset:device:activate",
	"POST /devices/:no/credential": "asset:device:credential",
	"GET /devices/:no/shadow":      "asset:device:list",
	"POST /devices/commands":       "asset:device:cmd",
	"GET /device-cmds":             "asset:device:cmd",
	"GET /devices/:no/topology":    "asset:device:list",
	"PUT /devices/:no/topology":    "asset:device:topology",
	"GET /firmwares":               "asset:ota:firmware",
	"POST /firmwares":              "asset:ota:firmware",
	"GET /ota-tasks":               "asset:ota:task",
	"GET /ota-tasks/:no":           "asset:ota:task",
	"POST /ota-tasks":              "asset:ota:task",
	"POST /ota-tasks/:no/rollback": "asset:ota:rollback",
	// S6-02 场站域（asset:station）
	"GET /stations":              "asset:station:list",
	"GET /stations/:no":          "asset:station:list",
	"POST /stations":             "asset:station:create",
	"POST /stations/:no/devices": "asset:station:bind",
	"GET /stations/:no/devices":  "asset:station:list",
	"GET /stations/:no/topology": "asset:station:list",
	"PUT /stations/:no/topology": "asset:station:topology",
	"GET /stations/:no/monitor":  "asset:station:list",
	"GET /stations/:no/staff":    "asset:station:staff",
	"POST /stations/:no/staff":   "asset:station:staff",
	"DELETE /stations/:no/staff": "asset:station:staff",
	"GET /device-station":        "asset:station:list",
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

// isBizNo 业务单号判定（S5 交易域前缀 + 纯数字）。
func isBizNo(s string) bool {
	prefixes := []string{"ORD", "RET", "SHP", "PAY", "RFD", "INV", "CTR", "CLM", "EXT", "WAR"}
	for _, p := range prefixes {
		if len(s) > len(p) && s[:len(p)] == p {
			for i := len(p); i < len(s); i++ {
				if s[i] < '0' || s[i] > '9' {
					return false
				}
			}
			return true
		}
	}
	return false
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
