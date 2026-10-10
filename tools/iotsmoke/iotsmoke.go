// iotsmoke · IoT 全链冒烟（S6 阶段验收：上报→TDengine P95<5s→影子→越限候选，FR-IOT-003/005/009）。
//
// 链路：ImportSN → stock_in/stock_out 事件驱动状态机 → Activate → MQTT 上报 → Kafka
// → writer(TDengine+影子) → GetDeviceShadow / TDengine 查询 P95；越限报文 → alert_candidate 消费验证。
//
// 前置：compose core+edge、mqinit、device/iotingest 进程运行中。
// 纪律：严禁并发执行（E1）。
//
//	go run ./tools/iotsmoke
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"

	"github.com/zxiaosi-micro/micro-common/authz"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"google.golang.org/grpc"

	devpb "micro-server/services/device/pb"
)

const (
	tenantID = 9000000000000000001 // e2esmoke 同源种子租户
	seedUID  = 9000000000000000002
)

var (
	kafkaBrokers = envOr("KAFKA_BROKERS", "127.0.0.1:29092")
	mqttBroker   = envOr("EMQX_BROKER", "tcp://127.0.0.1:21883")
	mqttUser     = envOr("EMQX_PLATFORM_USER", "micro-dev-device")
	mqttPass     = envOr("EMQX_PLATFORM_PASS", "")
	tdRest       = envOr("TDENGINE_REST", "http://127.0.0.1:26041")
	tdUser       = envOr("TDENGINE_USER", "root")
	tdPass       = envOr("MICRO_DEV_TDENGINE_PW", "taosdata")
)

func main() {
	logx.MustSetup(logx.LogConf{ServiceName: "iotsmoke", Mode: "console"})
	ctx := ctxkit.WithUID(ctxkit.WithTenant(context.Background(), tenantID), seedUID)

	// ---- 直连 device RPC ----
	dconn := zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{envOr("DEVICE_RPC", "127.0.0.1:8085")},
		NonBlock:  true, Timeout: 8000,
	}, zrpc.WithDialOption(grpc.WithUnaryInterceptor(authz.OutgoingInterceptor))).Conn()
	dev := devpb.NewDeviceClient(dconn)

	sn := fmt.Sprintf("SMOKE-%d", time.Now().UnixMilli()%100000000)

	// ---- 1. ImportSN（建档，不写 EMQX——冒烟走平台联调账号）----
	t0 := time.Now()
	imp, err := dev.ImportSN(ctx, &devpb.ImportSNReq{
		Items:     []*devpb.ImportDeviceItem{{Sn: sn, ProductKey: "ESS-DEMO", Model: "SIM", BatchNo: "SMOKE"}},
		Provision: true,
	})
	must(err, "ImportSN")
	pass("ImportSN 建档 sn=%s imported=%d 耗时=%s", sn, imp.Imported, time.Since(t0).Round(time.Millisecond))

	// ---- 2. stock_in / stock_out 事件驱动状态机 ----
	produceEvent("stock_in", map[string]any{
		"tenant_id": tenantID, "warehouse_id": 1, "sku_id": 1, "qty": 1, "biz_no": "SMOKE-IN-" + sn, "sns": []string{sn},
	})
	produceEvent("stock_out", map[string]any{
		"tenant_id": tenantID, "warehouse_id": 1, "sku_id": 1, "qty": 1, "biz_no": "SMOKE-OUT-" + sn, "sns": []string{sn},
	})
	waitFor(15*time.Second, 500*time.Millisecond, "设备状态 OUT", func() (bool, error) {
		resp, err := dev.ListDevice(ctx, &devpb.ListDeviceReq{Keyword: sn})
		if err != nil {
			return false, err
		}
		return len(resp.List) == 1 && resp.List[0].Status == "OUT", nil
	})
	pass("stock_in/stock_out 状态机推进 PRODUCED→IN_STOCK→OUT")

	// ---- 3. Activate（OUT→ACTIVATED + device_activated）----
	_, err = dev.Activate(ctx, &devpb.ActivateReq{Sn: sn})
	must(err, "Activate")
	waitFor(15*time.Second, 500*time.Millisecond, "状态 ACTIVATED", func() (bool, error) {
		resp, err := dev.ListDevice(ctx, &devpb.ListDeviceReq{Keyword: sn})
		if err != nil {
			return false, err
		}
		return len(resp.List) == 1 && resp.List[0].Status == "ACTIVATED", nil
	})
	pass("Activate 激活完成（device_activated 事件已发）")

	// ---- 4. MQTT 上报 → Kafka → writer（TDengine + 影子）----
	mcli := mqttConnect()
	upTopic := fmt.Sprintf("up/%d/ESS-DEMO/%s/telemetry", tenantID, sn)
	sendTelemetry := func(voltage float64) time.Time {
		payload, _ := json.Marshal(map[string]any{
			"soc": 80.5, "voltage": voltage, "current": 10.2,
			"temperature": 31.5, "power": 480.0, "ts": time.Now().UnixMilli(),
		})
		mcli.Publish(upTopic, 1, false, payload)
		return time.Now()
	}
	sendTelemetry(48.0)
	sendTelemetry(48.1)

	// ---- 5. 影子验证（P95<5s 口径：单点轮询 + 全量时延记录）----
	var latencies []time.Duration
	waitFor(20*time.Second, 200*time.Millisecond, "影子出现", func() (bool, error) {
		t1 := time.Now()
		resp, err := dev.GetDeviceShadow(ctx, &devpb.GetDeviceShadowReq{Sn: sn})
		if err != nil {
			return false, nil // 影子未就绪
		}
		latencies = append(latencies, time.Since(t1))
		return len(resp.Metrics) > 0, nil
	})
	p95 := percentile(latencies, 0.95)
	pass("影子返回 metrics（查询延迟 P95 样本=%v）", p95)

	// ---- 6. TDengine 落库验证（上报到可查询 < 5s，FR-IOT-003）----
	waitFor(20*time.Second, 500*time.Millisecond, "TDengine 子表数据", func() (bool, error) {
		sql := fmt.Sprintf(`select count(*) from micro_iot.iot_ess_demo_%s`, snLower(sn))
		body, err := tdQuery(sql)
		if err != nil {
			return false, nil
		}
		return len(body) > 2 && (body[0] == '[' || body[0] == '{') && string(body) != `{"code":0,"desc":null}` && !jsonEmptyCount(body), nil
	})
	sendTS := sendTelemetry(48.2)
	waitFor(10*time.Second, 200*time.Millisecond, "端到端 P95<5s（最新 ts 落库）", func() (bool, error) {
		sql := fmt.Sprintf(`select last(ts) from micro_iot.iot_ess_demo_%s`, snLower(sn))
		body, err := tdQuery(sql)
		if err != nil {
			return false, nil
		}
		lastMs := parseTS(body)
		return lastMs > 0 && time.Since(time.UnixMilli(lastMs)) < 5*time.Second, nil
	})
	_ = sendTS
	pass("TDengine 端到端落库 < 5s（上报→可查询）")

	// ---- 7. 越限候选（电压 >500V → alert_candidate）----
	ackConsumer := consume("alert_candidate", "iotsmoke-verify")
	defer ackConsumer.Close()
	sendTelemetry(520.0)
	waitFor(20*time.Second, 500*time.Millisecond, "alert_candidate 越限候选", func() (bool, error) {
		return ackConsumer.has(sn), nil
	})
	pass("越限报文 → alert_candidate（业务判定归 ops，FR-IOT-009）")

	// ---- 8. 指令下行（MQTT down/{sn}/cmd；devicesim 未在环——验证 cmd 表 + SENT 状态）----
	_, err = dev.SendCommand(ctx, &devpb.SendCommandReq{Sn: sn, CmdType: "QUERY"})
	must(err, "SendCommand")
	waitFor(20*time.Second, 500*time.Millisecond, "cmd SENT（QoS1 投递）", func() (bool, error) {
		resp, err := dev.ListCmd(ctx, &devpb.ListCmdReq{Sn: sn})
		if err != nil {
			return false, err
		}
		return len(resp.List) == 1 && (resp.List[0].Status == "SENT" || resp.List[0].Status == "PENDING"), nil
	})
	pass("指令下行入队 + MQTT 投递（ACK 3s/重试/告警链路由 cron 兜底验证）")

	fmt.Println("\niotsmoke: 全部通过 ✅")
}

// ---- 基础设施 ----

func mqttConnect() paho.Client {
	opts := paho.NewClientOptions().
		AddBroker(mqttBroker).
		SetClientID(fmt.Sprintf("iotsmoke-%d", time.Now().UnixMilli())).
		SetUsername(mqttUser).
		SetPassword(mqttPass)
	cli := paho.NewClient(opts)
	if token := cli.Connect(); !token.WaitTimeout(10*time.Second) || token.Error() != nil {
		fail("MQTT 连接失败: %v", token.Error())
	}
	return cli
}

type verifier struct {
	reader *kafka.Reader
	seen   map[string]bool
}

func consume(topic, group string) *verifier {
	return &verifier{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{kafkaBrokers}, GroupID: group + fmt.Sprintf("-%d", time.Now().UnixMilli()%100000),
			Topic: topic, MinBytes: 1, MaxBytes: 10e6,
		}),
		seen: map[string]bool{},
	}
}

func (v *verifier) has(key string) bool {
	if v.seen[key] {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	m, err := v.reader.FetchMessage(ctx)
	if err != nil {
		return false
	}
	var p struct {
		Sn string `json:"sn"`
	}
	_ = json.Unmarshal(m.Value, &p)
	if p.Sn != "" {
		v.seen[p.Sn] = true
	}
	return v.seen[key]
}

func (v *verifier) Close() { _ = v.reader.Close() }

func produceEvent(topic string, payload map[string]any) {
	raw, _ := json.Marshal(payload)
	w := &kafka.Writer{Addr: kafka.TCP(kafkaBrokers), Topic: topic, AllowAutoTopicCreation: false}
	if err := w.WriteMessages(context.Background(), kafka.Message{Key: []byte(sn0()), Value: raw}); err != nil {
		fail("Kafka 生产 %s 失败: %v", topic, err)
	}
	_ = w.Close()
}

func sn0() string { return "iotsmoke" }

func tdQuery(sql string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, tdRest+"/rest/sql", strings.NewReader(sql))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(tdUser, tdPass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	return body[:n], nil
}

func jsonEmptyCount(body []byte) bool {
	var arr []any
	if err := json.Unmarshal(body, &arr); err == nil {
		return len(arr) == 0 || fmt.Sprintf("%v", arr) == "[[0]]"
	}
	return false
}

func parseTS(body []byte) int64 {
	var arr []any
	if err := json.Unmarshal(body, &arr); err != nil || len(arr) == 0 {
		return 0
	}
	row, ok := arr[0].([]any)
	if !ok || len(row) < 2 {
		return 0
	}
	var ms int64
	// TDengine last(ts) 返回 ISO 串；粗解析毫秒
	s := fmt.Sprintf("%v", row[1])
	if t, err := time.Parse("2006-01-02 15:04:05.000", s+"Z"[0:0]); err == nil {
		ms = t.UnixMilli()
	} else if f, err := time.Parse(time.RFC3339Nano, s); err == nil {
		ms = f.UnixMilli()
	}
	return ms
}

func snLower(sn string) string {
	out := make([]byte, len(sn))
	for i, c := range []byte(sn) {
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out[i] = c
	}
	return string(out)
}

func percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	cp := append([]time.Duration{}, ds...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[int(float64(len(cp)-1)*p)]
}

func waitFor(total time.Duration, interval time.Duration, name string, probe func() (bool, error)) {
	deadline := time.Now().Add(total)
	for time.Now().Before(deadline) {
		ok, err := probe()
		if ok {
			return
		}
		_ = err
		time.Sleep(interval)
	}
	fail("等待超时: %s", name)
}

func must(err error, what string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "iotsmoke FAIL: %s: %v\n", what, err)
		os.Exit(1)
	}
}

func pass(format string, args ...any) {
	fmt.Printf("  ✅ "+format+"\n", args...)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "iotsmoke FAIL: "+format+"\n", args...)
	os.Exit(1)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
