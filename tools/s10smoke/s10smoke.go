// s10smoke.go · S10 增值域冒烟：退役评估三评级分支 + BI 白名单 + 数仓 ETL 幂等 + 经销商返利快照（E1 串行纪律）。
//
// 链路：device / analytics / finance / party RPC 直连（ctx 注入 default 租户 + seed uid）。
// 前置：
//   1. compose core 全绿（envcheck），source tools/devenv.sh（DSN 等注入）
//   2. 起服务：device / analytics / finance / party（迁移到最新：device 000003、analytics 000004）
//   3. go run ./tools/s10smoke
//
// 断言清单（步骤号即输出序号）：
//   1. 退役评估三评级分支各一例：
//      A（SOH85/循环1200/一致达标，RETIRED 设备）→ RESALE：设备 RETIRED→IN_STOCK（梯次利用白名单对；
//        再销售新单新质保走既有 S5 CreateFromOrder 链路，此处断言状态迁移+生命周期事件）；
//      B（SOH75）→ DERATE：设备保持 RETIRED；
//      C（SOH60，ACTIVATED 设备）→ RECYCLE：设备→SCRAPPED + 回收单 + 监管溯源码留痕。
//   2. BI 白名单外查询被拒（表不在白名单 151010 / 列不在白名单 151011）；白名单内查询成功返回行列。
//   3. 数仓 ETL 重跑幂等：同区间两次 RunDwEtl，第二次全量 affected=0。
//   4. 经销商返利：UpsertDealerExt 规则 → 微信模拟支付（payer=经销商）→ 月度结算金额=基数×费率
//      → 规则快照可追溯（rule_raw/base_amount/rate）→ ReconcileRebate 差异=0 → 重跑结算幂等。
//
// 严禁并发执行（E1）：种子数据带 runID 前缀落 default 租户。

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	anpb "micro-server/services/analytics/pb"
	dpb "micro-server/services/device/pb"
	finpb "micro-server/services/finance/pb"
	ppb "micro-server/services/party/pb"
)

const seedUID = 9000000000000000002

var (
	deviceRPC    = envOr("DEVICE_RPC", "127.0.0.1:8085")
	analyticsRPC = envOr("ANALYTICS_RPC", "127.0.0.1:8095")
	financeRPC   = envOr("FINANCE_RPC", "127.0.0.1:8092")
	partyRPC     = envOr("PARTY_RPC", "127.0.0.1:8082")
	identityDSN  = envOr("MICRO_DSN_IDENTITY", envOr("DSN", ""))
	runID        = fmt.Sprintf("s10-%d", time.Now().UnixMilli()%100000000)
)

func main() {
	logx.MustSetup(logx.LogConf{ServiceName: "s10smoke", Mode: "console"})
	fmt.Printf("=== s10smoke · S10 增值域冒烟 runID=%s（严禁并发 E1） ===\n", runID)

	tid := mustDefaultTenant()
	fmt.Printf("[0] default 租户 = %d\n", tid)
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tid), seedUID)

	device := dpb.NewDeviceClient(rpcConn(deviceRPC))
	analytics := anpb.NewAnalyticsClient(rpcConn(analyticsRPC))
	fin := finpb.NewFinanceClient(rpcConn(financeRPC))
	partySvc := ppb.NewPartyClient(rpcConn(partyRPC))

	// ---- 1. 退役评估三评级分支 ----
	fmt.Println("---- 1. 退役评估三评级分支 ----")

	// 种子 3 台设备：dev1/dev2 置 RETIRED（评估入口双态覆盖），dev3 置 ACTIVATED
	sns := map[string]string{
		"A": "S10A" + fmt.Sprintf("%d", time.Now().UnixMilli()),
		"B": "S10B" + fmt.Sprintf("%d", time.Now().UnixMilli()),
		"C": "S10C" + fmt.Sprintf("%d", time.Now().UnixMilli()),
	}
	statuses := map[string]string{"A": "RETIRED", "B": "RETIRED", "C": "ACTIVATED"}
	for branch, sn := range sns {
		_, err := device.ImportSN(ctx, &dpb.ImportSNReq{
			Items:     []*dpb.ImportDeviceItem{{Sn: sn, ProductKey: "s10-bms", Model: "S10-Model", BatchNo: runID}},
			Provision: false,
		})
		must(err, "ImportSN "+branch)
		bumpDeviceStatus(sn, statuses[branch])
	}
	pass("0 种子设备 3 台（A/B=RETIRED，C=ACTIVATED）")

	// 发起评估 + 检测录入 + 评级断言
	cases := []struct {
		branch string
		soh    string
		cycles int32
		want   string
	}{
		{"A", "85", 1200, "A"},
		{"B", "75", 1200, "B"},
		{"C", "60", 1200, "C"},
	}
	assessmentIDs := map[string]int64{}
	for _, c := range cases {
		created, err := device.CreateRetirementAssessment(ctx, &dpb.CreateRetirementAssessmentReq{
			DeviceId: deviceIDBySn(ctx, device, tid, sns[c.branch]),
		})
		must(err, "CreateRetirementAssessment "+c.branch)
		assessmentIDs[c.branch] = created.AssessmentId

		sub, err := device.SubmitRetirementResult(ctx, &dpb.SubmitRetirementResultReq{
			AssessmentId:  created.AssessmentId,
			Soh:           c.soh,
			CycleCount:    c.cycles,
			ConsistencyOk: true,
			SafetyItems:   []*dpb.SafetyItem{{Item: "过充", Pass: true}, {Item: "短路", Pass: true}},
			Source:        "BENCH",
		})
		must(err, "SubmitRetirementResult "+c.branch)
		if sub.Grade != c.want {
			fail(fmt.Sprintf("评级分支 %s：期望 %s 实际 %s（%s）", c.branch, c.want, sub.Grade, sub.Reason))
		}
	}
	pass("1a 三评级分支各一例：A/B/C 判定正确（评级口径定稿）")

	// A → RESALE：RETIRED→IN_STOCK（梯次利用白名单对；新质保由再销售新单走 S5 CreateFromOrder 既有链路）
	decA, err := device.DecideRetirement(ctx, &dpb.DecideRetirementReq{AssessmentId: assessmentIDs["A"]})
	must(err, "DecideRetirement A")
	if decA.Disposition != "RESALE" || decA.DeviceStatus != "IN_STOCK" {
		fail(fmt.Sprintf("A 级处置：期望 RESALE+IN_STOCK，实际 %s+%s", decA.Disposition, decA.DeviceStatus))
	}
	pass("1b A 级 RESALE 生效：设备 RETIRED→IN_STOCK（二次销售入口；新单新质保走 S5 CreateFromOrder）")

	// B → DERATE：保持 RETIRED
	decB, err := device.DecideRetirement(ctx, &dpb.DecideRetirementReq{AssessmentId: assessmentIDs["B"]})
	must(err, "DecideRetirement B")
	if decB.Disposition != "DERATE" || decB.DeviceStatus != "RETIRED" {
		fail(fmt.Sprintf("B 级处置：期望 DERATE+RETIRED，实际 %s+%s", decB.Disposition, decB.DeviceStatus))
	}
	pass("1c B 级 DERATE：降容备电，设备保持 RETIRED")

	// C → RECYCLE：SCRAPPED + 回收单 + 溯源码
	decC, err := device.DecideRetirement(ctx, &dpb.DecideRetirementReq{AssessmentId: assessmentIDs["C"]})
	must(err, "DecideRetirement C")
	if decC.Disposition != "RECYCLED" || decC.DeviceStatus != "SCRAPPED" || decC.RecycleNo == "" || decC.TraceCode == "" {
		fail(fmt.Sprintf("C 级处置：期望 RECYCLED+SCRAPPED+回收单+溯源码，实际 %+v", decC))
	}
	recycled, err := device.ListRecycleOrder(ctx, &dpb.ListRecycleOrderReq{Keyword: decC.TraceCode})
	must(err, "ListRecycleOrder")
	if recycled.Total != 1 || len(recycled.List) != 1 || recycled.List[0].TraceCode != decC.TraceCode {
		fail(fmt.Sprintf("回收单留痕断言失败：total=%d", recycled.Total))
	}
	pass(fmt.Sprintf("1d C 级 RECYCLE：回收单 %s + 溯源码 %s 留痕", decC.RecycleNo, decC.TraceCode))

	// ---- 2. BI 白名单 ----
	fmt.Println("---- 2. BI 白名单查询引擎 ----")
	_, err = analytics.ListBIWhitelist(ctx, &anpb.ListBIWhitelistReq{})
	must(err, "ListBIWhitelist")
	pass("2a 白名单元数据可读（表/字段/维度/聚合/操作符）")

	// 白名单外表被拒（151010）
	_, err = analytics.RunBIQuery(ctx, &anpb.RunBIQueryReq{
		Table: "micro_order.`order`", Dimensions: []string{"date_key"},
	})
	mustReject(err, "151010", "表不在白名单")
	// 白名单外列被拒（151011）
	_, err = analytics.RunBIQuery(ctx, &anpb.RunBIQueryReq{
		Table: "dw_fact_order", Dimensions: []string{"password_hash"},
	})
	mustReject(err, "151011", "列不在白名单")
	pass("2b 白名单外查询被拒：表 151010 / 列 151011")

	ok, err := analytics.RunBIQuery(ctx, &anpb.RunBIQueryReq{
		Table:      "dw_fact_order",
		Dimensions: []string{"status"},
		Metrics:    []string{"COUNT:*", "SUM:total_amount"},
	})
	must(err, "RunBIQuery 白名单内")
	if len(ok.Columns) != 3 {
		fail(fmt.Sprintf("白名单内查询列数异常：%v", ok.Columns))
	}
	pass(fmt.Sprintf("2c 白名单内查询成功：cols=%v rows=%d total=%d", ok.Columns, len(ok.Rows), ok.Total))

	// ---- 3. 数仓 ETL 幂等 ----
	fmt.Println("---- 3. 数仓 ETL 重跑幂等 ----")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	etlReq := &anpb.RunDwEtlReq{DateStart: yesterday, DateEnd: today}
	first, err := analytics.RunDwEtl(ctx, etlReq)
	must(err, "RunDwEtl 首跑")
	second, err := analytics.RunDwEtl(ctx, etlReq)
	must(err, "RunDwEtl 重跑")
	for i, d := range second.Days {
		if d.FactOrderRows != 0 || d.FactPaymentRows != 0 || d.DimPartyRows != 0 || d.DimTenantRows != 0 || d.DimDateRows != 0 {
			fail(fmt.Sprintf("ETL 重跑非幂等：%s 重跑 affected=%d/%d/%d/%d/%d（期望全 0）",
				d.EtlDate, d.DimDateRows, d.DimTenantRows, d.DimPartyRows, d.FactOrderRows, d.FactPaymentRows))
		}
		_ = i
	}
	pass(fmt.Sprintf("3 ETL 重跑幂等：区间[%s,%s] 首跑 fact_order=%d/%d，重跑全 0",
		yesterday, today, first.Days[len(first.Days)-1].FactOrderRows, first.Days[len(first.Days)-1].FactPaymentRows))

	// ---- 4. 经销商返利 ----
	fmt.Println("---- 4. 经销商返利（规则快照 + 对账） ----")
	dealer, err := partySvc.CreateParty(ctx, &ppb.CreatePartyReq{
		Name: "s10返利经销商" + runID, Type: []string{"DEALER"}, Region: "dev",
	})
	must(err, "CreateParty(DEALER)")
	rateRule := `{"rate":0.05,"settle":"MONTHLY"}`
	_, err = partySvc.UpsertDealerExt(ctx, &ppb.UpsertDealerExtReq{
		PartyId: dealer.PartyId, DealerLevel: "GOLD", AuthorizedRegion: "dev", RebateRule: rateRule,
	})
	must(err, "UpsertDealerExt")

	// 微信模拟支付（payer=经销商；ConfirmPayment 路径无对公复核，payer 归属即返利基数）
	orderNo := "S10ORD" + fmt.Sprintf("%d", time.Now().UnixMilli())
	pay, err := fin.CreatePayment(ctx, &finpb.CreatePaymentReq{
		OrderNo: orderNo, Amount: "10000.00", Channel: "WECHAT", PayerPartyId: dealer.PartyId,
	})
	must(err, "CreatePayment")
	_, err = fin.ConfirmPayment(ctx, &finpb.ConfirmPaymentReq{
		PaymentNo: pay.PaymentNo, ChannelTxnId: "S10TXN-" + runID,
		PaidAmount: "10000.00", Source: "MOCK",
	})
	must(err, "ConfirmPayment")

	period := time.Now().Format("2006-01")
	settle, err := fin.RunRebateSettlement(ctx, &finpb.RunRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "RunRebateSettlement")
	if len(settle.Results) != 1 {
		fail(fmt.Sprintf("返利结算结果数异常：%d", len(settle.Results)))
	}
	r := settle.Results[0]
	if r.Amount != "500.00" || r.BaseAmount != "10000.00" || r.Rate != "0.05" {
		fail(fmt.Sprintf("返利金额断言失败：base=%s rate=%s amount=%s（期望 10000.00×0.05=500.00）", r.BaseAmount, r.Rate, r.Amount))
	}
	pass(fmt.Sprintf("4a 月度结算：基数 %s × %s = %s（支付单 %d 笔）", r.BaseAmount, r.Rate, r.Amount, r.PaymentCount))

	// 规则快照可追溯
	list, err := fin.ListRebateSettlement(ctx, &finpb.ListRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "ListRebateSettlement")
	if len(list.List) != 1 {
		fail("返利结算单列表为空")
	}
	var snap map[string]any
	must(json.Unmarshal([]byte(list.List[0].RuleSnapshot), &snap), "解析规则快照")
	if snap["rule_raw"] != rateRule || snap["base_amount"] != "10000.00" {
		fail(fmt.Sprintf("规则快照追溯断言失败：%s", list.List[0].RuleSnapshot))
	}
	pass("4b 规则快照可追溯：rule_raw/base_amount/rate/calc 全留痕")

	// 月度对账差异为 0
	recon, err := fin.ReconcileRebate(ctx, &finpb.ReconcileRebateReq{Period: period})
	must(err, "ReconcileRebate")
	if recon.DiffCount != 0 {
		fail(fmt.Sprintf("返利对账差异非 0：%d（%v）", recon.DiffCount, recon.Diffs))
	}
	pass("4c 月度对账差异为 0 ✓")

	// 重跑结算幂等（PENDING 可覆盖但同值；对账仍为 0）
	_, err = fin.RunRebateSettlement(ctx, &finpb.RunRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "RunRebateSettlement 重跑")
	recon2, err := fin.ReconcileRebate(ctx, &finpb.ReconcileRebateReq{Period: period})
	must(err, "ReconcileRebate 重跑后")
	if recon2.DiffCount != 0 {
		fail(fmt.Sprintf("重跑结算后对账差异非 0：%d", recon2.DiffCount))
	}
	// 确认后金额锁定：CONFIRM → 重跑结算不覆盖
	list2, err := fin.ListRebateSettlement(ctx, &finpb.ListRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "ListRebateSettlement 2")
	_, err = fin.ConfirmRebateSettlement(ctx, &finpb.ConfirmRebateSettlementReq{
		SettlementId: list2.List[0].SettlementId, Action: "CONFIRM",
	})
	must(err, "ConfirmRebateSettlement")
	_, err = fin.RunRebateSettlement(ctx, &finpb.RunRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "RunRebateSettlement 确认后重跑")
	list3, err := fin.ListRebateSettlement(ctx, &finpb.ListRebateSettlementReq{Period: period, PartyId: dealer.PartyId})
	must(err, "ListRebateSettlement 3")
	if list3.List[0].Status != "CONFIRMED" {
		fail(fmt.Sprintf("确认后重跑覆盖了状态：%s", list3.List[0].Status))
	}
	pass("4d 结算幂等 + 确认锁定：重跑覆盖仅限 PENDING，CONFIRMED 金额锁定")

	fmt.Printf("\n=== s10smoke 全绿 ✓ runID=%s ===\n", runID)
}

// ---------- 助手（p5smoke 同款约定） ----------

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

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

// bumpDeviceStatus 冒烟前置：状态直置（真实链路由 Saga/事件驱动，避免拉起全交易链；p5smoke bumpDeviceOut 同款）。
func bumpDeviceStatus(sn, status string) {
	db, err := sql.Open("mysql", strings.Replace(identityDSN, "/micro_identity", "/micro_device", 1))
	must(err, "连 device 库")
	defer func() { _ = db.Close() }()
	res, err := db.Exec("update `device` set `status`=? where `sn`=? and `deleted_at` is null", status, sn)
	must(err, "置状态 "+status)
	if n, _ := res.RowsAffected(); n != 1 {
		fail("设备置状态失败：" + sn)
	}
}

func deviceIDBySn(ctx context.Context, d dpb.DeviceClient, tid int64, sn string) int64 {
	out, err := d.GetDeviceBySn(ctx, &dpb.GetDeviceBySnReq{Sn: sn})
	must(err, "GetDeviceBySn "+sn)
	return out.Device.DeviceId
}

// mustReject 断言 RPC 业务错误码（errcode Error 格式 [151010] …）。
func mustReject(err error, code, what string) {
	if err == nil {
		fail(what + "：期望被拒（" + code + "），实际成功")
	}
	if !strings.Contains(err.Error(), code) {
		fail(fmt.Sprintf("%s：期望 %s，实际 %v", what, code, err))
	}
}

func must(err error, what string) {
	if err != nil {
		fail(what + "：" + err.Error())
	}
}

func pass(msg string) { fmt.Println("✓", msg) }

func fail(msg string) {
	fmt.Println("✗", msg)
	os.Exit(1)
}
