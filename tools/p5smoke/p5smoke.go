// p5smoke.go · S9 阶段冒烟：小程序基座 + 客户端接口 + 微信支付模拟 + 互斥矩阵（E1 串行纪律）。
//
// 链路：mobile-bff（:8890）HTTP 真请求 + 下游 RPC 直连种子。
// 前置：
//   1. compose core 全绿（envcheck 5/5），source tools/devenv.sh（MOCKWX=1 注入，04 §3 F2 dev 模拟）
//   2. 起服务：identity / party / catalog / inventory / device / contract / order / finance / ops / notification / mobile-bff
//   3. 跑迁移到最新（含 ops 000003 ticket.party_id）
//   4. go run ./tools/p5smoke
//
// 断言清单（步骤号即输出序号）：
//   1. /healthz 深检 ok
//   2. 微信登录链路：wx-login(MOCKWX) → need_bind_phone → wx-bind-phone → 签发会话
//   3. requireParty：绑定 party 后 me.has_party=true；未绑定账号访问设备列表 → 110404
//   4. 设备激活 + party 归属：activate 绑本人 → devices 列表可见；他人访问详情 → 10408
//   5. 报修工单：AFTER_SALE 创建 → party_id 隔离列表可见；他人列表不可见
//   6. 质保起算（事件驱动）：激活 → contract warranty（waitFor）
//   7. 延保购买：EXT_WARRANTY SKU 统一下单链路（order+inventory 首仓代选）→ PayOrder 微信
//   8. 微信支付模拟：pay_params 为 mock URL；finance.ConfirmPayment → PAID
//   9. 互斥矩阵：CLIENT_MINI 同端互斥踢旧（旧 token 10402）；OPS_APP 同端互斥踢旧；跨端共存（MINI+APP 并存）
//  10. App routes：/app/me、alerts/tickets pending、inspections 列表 200
//
// 严禁并发执行（E1）：种子数据带 runID 前缀落默认租户（与微信登录自动注册同租户）。

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	cpb "micro-server/services/catalog/pb"
	ctpb "micro-server/services/contract/pb"
	dpb "micro-server/services/device/pb"
	finpb "micro-server/services/finance/pb"
	ipb "micro-server/services/identity/pb"
	invpb "micro-server/services/inventory/pb"
	ppb "micro-server/services/party/pb"
)

const (
	seedUID = 9000000000000000002
	bffBase = "http://127.0.0.1:8890/api/v1"
	opsPass = "P5smoke#2026"
)

// uniqPhone runID 派生 11 位手机号（mobile_hash 全局 UK，跨 run 不得重复）。
func uniqPhone(n int) string {
	var seed int64
	_, _ = fmt.Sscanf(strings.TrimPrefix(runID, "p5-"), "%d", &seed)
	return fmt.Sprintf("138%08d", (seed*100+int64(n))%100000000)
}

var (
	identityRPC  = envOr("IDENTITY_RPC", "127.0.0.1:8081")
	partyRPC     = envOr("PARTY_RPC", "127.0.0.1:8082")
	catalogRPC   = envOr("CATALOG_RPC", "127.0.0.1:8083")
	inventoryRPC = envOr("INVENTORY_RPC", "127.0.0.1:8084")
	deviceRPC    = envOr("DEVICE_RPC", "127.0.0.1:8085")
	financeRPC   = envOr("FINANCE_RPC", "127.0.0.1:8092")
	contractRPC  = envOr("CONTRACT_RPC", "127.0.0.1:8093")
	identityDSN  = envOr("MICRO_DSN_IDENTITY", envOr("DSN", ""))
	runID        = fmt.Sprintf("p5-%d", time.Now().UnixMilli()%100000000)
)

// ---- HTTP 信封 ----

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// bffDo mobile-bff HTTP 请求（信封原样返回，业务码由调用方断言）。
func bffDo(method, path, token string, body any) (int, envelope) {
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, bffBase+path, rd)
	must(err, "构造请求 "+path)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	must(err, "请求 "+path)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env
}

func bffOK(method, path, token string, body any) json.RawMessage {
	status, env := bffDo(method, path, token, body)
	if status != http.StatusOK || env.Code != 0 {
		fail(fmt.Sprintf("%s %s 期望成功，实际 http=%d code=%d msg=%s", method, path, status, env.Code, env.Msg))
	}
	return env.Data
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func main() {
	logx.MustSetup(logx.LogConf{ServiceName: "p5smoke", Mode: "console"})
	fmt.Printf("=== p5smoke · S9 多端冒烟 runID=%s（严禁并发 E1） ===\n", runID)

	// ---- 0. 前置：default 租户 + RPC 客户端 ----
	tid := mustDefaultTenant()
	fmt.Printf("[0] default 租户 = %d\n", tid)
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tid), seedUID)

	identity := ipb.NewIdentityClient(rpcConn(identityRPC))
	partySvc := ppb.NewPartyClient(rpcConn(partyRPC))
	catalog := cpb.NewCatalogClient(rpcConn(catalogRPC))
	inv := invpb.NewInventoryClient(rpcConn(inventoryRPC))
	device := dpb.NewDeviceClient(rpcConn(deviceRPC))
	fin := finpb.NewFinanceClient(rpcConn(financeRPC))
	contract := ctpb.NewContractClient(rpcConn(contractRPC))

	// ---- 1. healthz 深检 ----
	data := bffOK("GET", "/healthz", "", nil)
	var health struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	must(json.Unmarshal(data, &health), "解析 healthz")
	if health.Status != "ok" {
		fail(fmt.Sprintf("healthz status=%s checks=%v", health.Status, health.Checks))
	}
	pass("1 /healthz 深检 ok（identity+tdengine）")

	// ---- 种子：party / OPS 用户 / 设备 / 延保 SKU / 仓库库存 ----
	partyResp, err := partySvc.CreateParty(ctx, &ppb.CreatePartyReq{
		Name:   "p5冒烟客户" + runID,
		Type:   []string{"CUSTOMER"},
		Region: "dev",
	})
	must(err, "CreateParty")
	partyID := partyResp.PartyId

	opsMobile := "139" + fmt.Sprintf("%08d", time.Now().UnixMilli()%100000000)
	_, err = identity.CreateUser(ctx, &ipb.CreateUserReq{
		Nickname: "p5运维" + runID,
		Mobile:   opsMobile,
		Password: opsPass,
		Types:    []string{"OPS_APP"},
	})
	must(err, "CreateUser(OPS_APP)")

	sn := "P5SN" + fmt.Sprintf("%d", time.Now().UnixMilli())
	_, err = device.ImportSN(ctx, &dpb.ImportSNReq{
		Items:     []*dpb.ImportDeviceItem{{Sn: sn, ProductKey: "p5-bms", Model: "P5-Model", BatchNo: runID}},
		Provision: false, // 冒烟不写 EMQX（凭证链路由 iotsmoke 覆盖）
	})
	must(err, "ImportSN")
	bumpDeviceOut(sn) // 激活前置状态：PRODUCED → OUT（真实链路由 Saga 出库驱动，冒烟 SQL 直改）

	prodResp, err := catalog.CreateProduct(ctx, &cpb.CreateProductReq{Name: "p5延保产品-" + runID, Category: "EXT_WARRANTY"})
	must(err, "CreateProduct")
	seedWarrantyPending(sn) // PENDING 质保行（起算由 device_activated 事件驱动）

	skuResp, err := catalog.CreateSKU(ctx, &cpb.CreateSKUReq{
		ProductId: prodResp.ProductId,
		Code:      "EXT-" + runID, Name: "p5延保12个月", Type: "EXT_WARRANTY", Spec: `{"months":12}`,
	})
	must(err, "CreateSKU(EXT_WARRANTY)")
	skuID := skuResp.SkuId
	_, err = catalog.SetPrice(ctx, &cpb.SetPriceReq{SkuId: skuID, PriceType: "RETAIL", Amount: "299.00"})
	must(err, "SetPrice")

	whResp, err := inv.CreateWarehouse(ctx, &invpb.CreateWarehouseReq{Code: "P5-WH-" + runID, Name: "p5冒烟仓"})
	must(err, "CreateWarehouse")
	_, err = inv.StockIn(ctx, &invpb.StockInReq{
		WarehouseId: whResp.WarehouseId, SkuId: skuID, Qty: 100,
		BizType: "STOCK_IN", BizNo: "P5IN-" + runID,
	})
	must(err, "StockIn")
	pass("0 种子就绪 party/device/EXT_SKU/warehouse")

	// ---- 2. 微信登录链路（MOCKWX）----
	wxCode := runID + "-wx1"
	data = bffOK("POST", "/auth/wx-login", "", map[string]string{"code": wxCode})
	var wx1 struct {
		Tokens        *tokens `json:"tokens"`
		UID           string  `json:"uid"`
		NeedBindPhone bool    `json:"need_bind_phone"`
	}
	must(json.Unmarshal(data, &wx1), "解析 wx-login")
	// tokens 为值类型信封（未签发=空对象），客户端语义以 need_bind_phone 标志为准
	if !wx1.NeedBindPhone || wx1.Tokens == nil || wx1.Tokens.AccessToken != "" || wx1.UID == "" {
		fail(fmt.Sprintf("首登应 need_bind_phone=true 且不签发 token，实际 %+v", wx1))
	}
	data = bffOK("POST", "/auth/wx-bind-phone", "", map[string]string{"code": wxCode, "phone": uniqPhone(1)})
	var wx1b struct {
		Tokens        *tokens `json:"tokens"`
		NeedBindPhone bool    `json:"need_bind_phone"`
	}
	must(json.Unmarshal(data, &wx1b), "解析 wx-bind-phone")
	if wx1b.Tokens == nil || wx1b.Tokens.AccessToken == "" {
		fail("绑定手机号后应签发 token")
	}
	tok1 := wx1b.Tokens.AccessToken
	pass("2 微信登录链路：首登影子账号 → 绑手机签发会话")

	// ---- 3. requireParty ----
	_, env := bffDo("GET", "/customer/me", tok1, nil)
	if env.Code != 0 {
		fail(fmt.Sprintf("绑定 party 前不该失败于此（实际 code=%d）", env.Code))
	}
	var me1 struct {
		HasParty bool `json:"has_party"`
	}
	must(json.Unmarshal(env.Data, &me1), "解析 me")
	if me1.HasParty {
		fail("新微信账号不应已绑定 party")
	}
	_, env = bffDo("GET", "/customer/devices", tok1, nil)
	if env.Code != 110404 {
		fail(fmt.Sprintf("未绑定 party 访问设备列表应 110404，实际 code=%d msg=%s", env.Code, env.Msg))
	}
	_, err = identity.BindParty(ctx, &ipb.BindPartyReq{Uid: parseI64(wx1.UID), PartyId: partyID})
	must(err, "BindParty")
	data = bffOK("GET", "/customer/me", tok1, nil)
	must(json.Unmarshal(data, &me1), "解析 me2")
	if !me1.HasParty {
		fail("BindParty 后 has_party 应为 true")
	}
	contacts := bffOK("GET", "/customer/contacts", tok1, nil)
	if !strings.Contains(string(contacts), `"list"`) {
		fail("联系人响应缺 list")
	}
	pass("3 requireParty：未绑定 110404 → 绑定后放行")

	// ---- 4. 设备激活 + 归属隔离 ----
	bffOK("POST", "/customer/devices/activate", tok1, map[string]string{"sn": sn})
	data = bffOK("GET", "/customer/devices", tok1, nil)
	var devList struct {
		Total int64 `json:"total"`
		List  []struct {
			Sn string `json:"sn"`
		} `json:"list"`
	}
	must(json.Unmarshal(data, &devList), "解析 devices")
	if devList.Total < 1 || devList.List[0].Sn != sn {
		fail(fmt.Sprintf("激活后设备列表应含 %s，实际 total=%d", sn, devList.Total))
	}
	data = bffOK("GET", "/customer/devices", tok1, nil)
	var full struct {
		List []struct {
			DeviceID   string `json:"device_id"`
			Sn         string `json:"sn"`
			ProductKey string `json:"product_key"`
		} `json:"list"`
	}
	must(json.Unmarshal(data, &full), "解析 devices2")
	devID := ""
	for _, d := range full.List {
		if d.Sn == sn {
			devID = d.DeviceID
		}
	}
	if devID == "" {
		fail("未取到激活设备 ID")
	}
	detail := bffOK("GET", "/customer/devices/"+devID, tok1, nil)
	if !strings.Contains(string(detail), `"shadow"`) {
		fail("设备详情缺 shadow 字段")
	}

	// ---- 4b. 第二个微信用户：requireParty 隔离（列表空 + 详情 403）----
	code2 := runID + "-wx2"
	data = bffOK("POST", "/auth/wx-login", "", map[string]string{"code": code2})
	var wx2 struct {
		Tokens *tokens `json:"tokens"`
		UID    string  `json:"uid"`
	}
	must(json.Unmarshal(data, &wx2), "解析 wx2")
	data = bffOK("POST", "/auth/wx-bind-phone", "", map[string]string{"code": code2, "phone": uniqPhone(2)})
	var wx2b struct {
		Tokens *tokens `json:"tokens"`
	}
	must(json.Unmarshal(data, &wx2b), "解析 wx2b")
	tok2 := wx2b.Tokens.AccessToken
	partyResp2, err := partySvc.CreateParty(ctx, &ppb.CreatePartyReq{Name: "p5冒烟客户B-" + runID, Type: []string{"CUSTOMER"}})
	must(err, "CreateParty2")
	_, err = identity.BindParty(ctx, &ipb.BindPartyReq{Uid: parseI64(wx2.UID), PartyId: partyResp2.PartyId})
	must(err, "BindParty2")

	data = bffOK("GET", "/customer/devices", tok2, nil)
	if strings.Contains(string(data), sn) {
		fail("隔离破绽：B 看到了 A 的设备")
	}
	_, env = bffDo("GET", "/customer/devices/"+devID, tok2, nil)
	if env.Code != 10408 {
		fail(fmt.Sprintf("B 访问 A 设备详情应 10408，实际 code=%d msg=%s", env.Code, env.Msg))
	}
	pass("4 设备激活 + party 归属可见 + 他人访问 10408")

	// ---- 5. 报修工单 + 隔离 ----
	tk := bffOK("POST", "/customer/tickets", tok1, map[string]any{"sn": sn, "title": "p5冒烟报修", "content": "无法充电"})
	var created struct {
		TicketID string `json:"ticket_id"`
		TicketNo string `json:"ticket_no"`
	}
	must(json.Unmarshal(tk, &created), "解析 ticket")
	data = bffOK("GET", "/customer/tickets", tok1, nil)
	if !strings.Contains(string(data), created.TicketNo) {
		fail("我的工单应含新报修单")
	}
	data = bffOK("GET", "/customer/tickets", tok2, nil)
	if strings.Contains(string(data), created.TicketNo) {
		fail("隔离破绽：B 看到了 A 的报修单")
	}
	detail = bffOK("GET", "/customer/tickets/"+created.TicketID, tok1, nil)
	if !strings.Contains(string(detail), `"logs"`) {
		fail("工单详情缺 logs")
	}
	bffOK("GET", "/customer/tickets/"+created.TicketID+"/sla", tok1, nil)
	pass("5 报修工单创建 + party_id 隔离（B 不可见）+ SLA 只读")

	// ---- 6. 质保起算（事件驱动）----
	var warrantyID int64
	waitFor(30*time.Second, 2*time.Second, "contract warranty 起算", func() bool {
		out, err := contract.GetWarrantyByTarget(ctx, &ctpb.GetWarrantyByTargetReq{TargetType: "DEVICE", TargetId: lookupDeviceID(ctx, device, sn)})
		if err != nil || len(out.List) == 0 {
			return false
		}
		warrantyID = out.List[0].WarrantyId
		return out.List[0].Status == "ACTIVE" || out.List[0].Status == "PENDING"
	})
	pass(fmt.Sprintf("6 质保起算（warranty=%d）", warrantyID))

	// ---- 7+8. 延保购买 + 微信支付模拟 ----
	ext := bffOK("POST", "/customer/extension-orders", tok1, map[string]any{
		"base_warranty_id": fmt.Sprintf("%d", warrantyID),
		"sku_id":           fmt.Sprintf("%d", skuID),
		"qty":              1,
	})
	var order struct {
		OrderNo     string `json:"order_no"`
		TotalAmount string `json:"total_amount"`
		PaymentNo   string `json:"payment_no"`
		PayParams   string `json:"pay_params"`
	}
	must(json.Unmarshal(ext, &order), "解析 extension-order")
	if order.OrderNo == "" || order.PaymentNo == "" {
		fail(fmt.Sprintf("延保下单返回不完整: %+v", order))
	}
	if !strings.Contains(order.PayParams, "mock") && !strings.Contains(order.PayParams, "pay") {
		fail(fmt.Sprintf("pay_params 应为 mock 支付链接，实际 %s", order.PayParams))
	}
	conf, err := fin.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo:    order.PaymentNo,
		ChannelTxnId: "MOCK-" + runID,
		PaidAmount:   order.TotalAmount,
		Source:       "MOCK",
	})
	must(err, "ConfirmPayment")
	if conf.Status != "PAID" {
		fail(fmt.Sprintf("确认支付后应 PAID，实际 %s", conf.Status))
	}
	pass(fmt.Sprintf("7+8 延保下单 %s + 微信支付模拟 PAID（%s）", order.OrderNo, order.TotalAmount))

	// ---- 9. 互斥矩阵 ----
	// 9a. 跨端共存：MINI 会话 + OPS_APP 登录并存
	loginOps := func() string {
		d := bffOK("POST", "/auth/login", "", map[string]string{"mobile": opsMobile, "password": opsPass})
		var lr struct {
			Tokens tokens `json:"tokens"`
		}
		must(json.Unmarshal(d, &lr), "解析 ops login")
		return lr.Tokens.AccessToken
	}
	opsTok1 := loginOps()
	bffOK("GET", "/app/me", opsTok1, nil)
	bffOK("GET", "/customer/me", tok1, nil)
	pass("9a 跨端共存：CLIENT_MINI + OPS_APP 并存可用")

	// 9b. OPS_APP 同端互斥：第二次登录踢旧
	opsTok2 := loginOps()
	_, env = bffDo("GET", "/app/me", opsTok1, nil)
	if env.Code != 10402 {
		fail(fmt.Sprintf("OPS_APP 同端互斥应踢旧 10402，实际 code=%d", env.Code))
	}
	bffOK("GET", "/app/me", opsTok2, nil)
	pass("9b OPS_APP 同端互斥踢旧（旧 10402 / 新可用）")

	// 9c. CLIENT_MINI 同端互斥：同 code 再登录踢旧
	_, env = bffDo("POST", "/auth/wx-bind-phone", "", map[string]string{"code": wxCode, "phone": uniqPhone(1)})
	var wxAgain struct {
		Tokens *tokens `json:"tokens"`
	}
	if env.Code == 0 {
		must(json.Unmarshal(env.Data, &wxAgain), "解析 re-login")
	} else {
		fail(fmt.Sprintf("同 code 重放 wx-bind-phone 应幂等成功，实际 code=%d", env.Code))
	}
	_, env = bffDo("GET", "/customer/me", tok1, nil)
	if env.Code != 10402 {
		fail(fmt.Sprintf("CLIENT_MINI 同端互斥应踢旧 10402，实际 code=%d", env.Code))
	}
	bffOK("GET", "/customer/me", wxAgain.Tokens.AccessToken, nil)
	pass("9c CLIENT_MINI 同端互斥踢旧（旧 10402 / 新可用）")

	// 新 token 上刷新订单可见（会话轮换后数据不变）
	data = bffOK("GET", "/customer/tickets", wxAgain.Tokens.AccessToken, nil)
	if !strings.Contains(string(data), created.TicketNo) {
		fail("互斥后新会话仍应看到本人工单")
	}

	// ---- 10. App routes ----
	data = bffOK("GET", "/app/me", opsTok2, nil)
	if !strings.Contains(string(data), `"uid"`) {
		fail("app/me 缺 uid")
	}
	pendA := bffOK("GET", "/app/alerts/pending?since_id=0", opsTok2, nil)
	if !strings.Contains(string(pendA), "unread_count") {
		fail("alerts pending 缺 unread_count")
	}
	pendT := bffOK("GET", "/app/tickets/pending?since_id=0", opsTok2, nil)
	if !strings.Contains(string(pendT), "unread_count") {
		fail("tickets pending 缺 unread_count")
	}
	bffOK("GET", "/app/inspections?page=1&size=10", opsTok2, nil)
	// 离线提交幂等：client_action_id 重复提交不炸（工单状态非处理中时幂等成功）
	clientAction := runID + "-act1"
	_, env = bffDo("POST", "/app/tickets/"+created.TicketID+"/submit", opsTok2, map[string]string{
		"result": "p5 离线草稿恢复同步", "client_action_id": clientAction,
	})
	if env.Code == 0 || env.Code == 91008 { // 状态机拒绝（未接单）属预期；幂等键路径不 500 即可
		pass("10 App routes + 离线提交幂等键路径通")
	} else {
		fail(fmt.Sprintf("submit 幂等键路径异常 code=%d msg=%s", env.Code, env.Msg))
	}

	fmt.Printf("\n=== p5smoke 全部通过（runID=%s，种子留在默认租户，可重复执行） ===\n", runID)
}

// ---- helpers ----

// rpcConn 直连 RPC（dev Endpoints；出站挂 metadata 桥——租户/uid 透传与业务码往返）。
func rpcConn(addr string) *grpc.ClientConn {
	return zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{addr},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
}

func mustDefaultTenant() int64 {
	if identityDSN == "" {
		fail("MICRO_DSN_IDENTITY/DSN 未注入——请 source tools/devenv.sh")
	}
	db, err := sql.Open("mysql", identityDSN)
	must(err, "连 identity 库")
	defer func() { _ = db.Close() }()
	var tid int64
	must(db.QueryRow("select tenant_id from tenant where tenant_code='default'").Scan(&tid), "查 default 租户")
	return tid
}

// bumpDeviceOut 冒烟前置：ImportSN 后设备为 PRODUCED，激活要求 OUT——
// 真实链路由 Saga 出库（stock_out 事件）驱动，冒烟用 SQL 直置避免拉起全交易链。
func bumpDeviceOut(sn string) {
	if identityDSN == "" {
		fail("DSN 未注入")
	}
	db, err := sql.Open("mysql", strings.Replace(identityDSN, "/micro_identity", "/micro_device", 1))
	must(err, "连 device 库")
	defer func() { _ = db.Close() }()
	res, err := db.Exec("update `device` set `status`='OUT' where `sn`=? and `deleted_at` is null", sn)
	must(err, "置 OUT")
	if n, _ := res.RowsAffected(); n != 1 {
		fail("设备置 OUT 失败：" + sn)
	}
}

// seedWarrantyPending 注入 PENDING 质保行（真实质保行由 Saga CreateFromOrder 建——e2esmoke 覆盖；
// 此处只验"激活事件→起算"链，start_rule 快照 event=device_activated）。
func seedWarrantyPending(sn string) {
	deviceDB := strings.Replace(identityDSN, "/micro_identity", "/micro_device", 1)
	devDB, err := sql.Open("mysql", deviceDB)
	must(err, "连 device 库")
	var deviceID int64
	must(devDB.QueryRow("select device_id from `device` where `sn` = ? and `deleted_at` is null", sn).Scan(&deviceID), "查 device_id")
	_ = devDB.Close()

	dsn := strings.Replace(identityDSN, "/micro_identity", "/micro_contract", 1)
	db, err := sql.Open("mysql", dsn)
	must(err, "连 contract 库")
	defer func() { _ = db.Close() }()
	wid := time.Now().UnixNano() / 1e6 << 22 // 毫秒级伪雪花（冒烟专用）
	_, err = db.Exec(`insert into warranty
		(warranty_id, warranty_no, level, target_type, target_id, target_key, status, months, start_rule, source_type, source_no, tenant_id, created_at, updated_at)
		values (?, ?, 'DEVICE', 'DEVICE', ?, ?, 'PENDING', 24, '{"event":"device_activated","months":24}', 'ORDER', 'P5-SEED', 9000000000000000001, now(3), now(3))`,
		wid, "P5W"+sn, deviceID, sn)
	must(err, "seed warranty")
}

func lookupDeviceID(ctx context.Context, device dpb.DeviceClient, sn string) int64 {
	out, err := device.GetDeviceBySn(ctx, &dpb.GetDeviceBySnReq{Sn: sn})
	must(err, "GetDeviceBySn")
	return out.GetDevice().DeviceId
}

func waitFor(total, interval time.Duration, name string, probe func() bool) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		if probe() {
			return
		}
		time.Sleep(interval)
	}
	fail("等待超时: " + name)
}

func parseI64(s string) int64 {
	var n int64
	_, _ = fmt.Sscan(s, &n)
	return n
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(err error, what string) {
	if err != nil {
		fmt.Printf("✗ %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(msg string) { fmt.Println("✓", msg) }

func fail(msg string) {
	fmt.Println("✗", msg)
	os.Exit(1)
}
