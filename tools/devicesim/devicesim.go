// devicesim · 设备协议标准报文模拟器（S6-03，ADR-19：协议标准+模拟器全链=验收口径）。
//
// 模式：
//
//	go run ./tools/devicesim -n 10 -rate 1            # 10 台 × 每台 1 条/s 稳定上报
//	go run ./tools/devicesim -n 2000 -rate 5 -dur 60s # 2000 台压测段
//
// 报文：up/{tenant}/{pk}/{sn}/telemetry 五指标 JSON（soc/voltage/current/temperature/power）；
// 同时订阅 down/{sn}/cmd 收指令回执 up/{tenant}/{pk}/{sn}/ack（cmd.ack 事件源）。
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

type deviceSim struct {
	sn     string
	topic  string
	client paho.Client
	rate   int
	stop   chan struct{}
}

func main() {
	n := flag.Int("n", 5, "模拟设备数")
	rate := flag.Int("rate", 1, "每台每秒上报条数")
	dur := flag.Duration("dur", 0, "运行时长（0=直到 Ctrl+C）")
	broker := flag.String("broker", envOr("EMQX_BROKER", "tcp://127.0.0.1:21883"), "EMQX broker")
	user := flag.String("user", envOr("EMQX_PLATFORM_USER", "micro-dev-device"), "MQTT 用户（dev 平台联调账号）")
	pass := flag.String("pass", envOr("EMQX_PLATFORM_PASS", ""), "MQTT 密码")
	tenant := flag.Int64("tenant", 1, "租户 ID")
	pk := flag.String("pk", "ESS-DEMO", "product_key")
	snPrefix := flag.String("prefix", "DEV", "SN 前缀")
	flag.Parse()

	fmt.Printf("devicesim: %d 台 × %d 条/s → %s up/%d/%s/<sn>/telemetry\n", *n, *rate, *broker, *tenant, *pk)

	stop := make(chan struct{})
	sims := make([]*deviceSim, 0, *n)
	for i := 0; i < *n; i++ {
		sn := *snPrefix + "-" + base64.RawURLEncoding.EncodeToString(randBytes(4)) + "-" + strconv.Itoa(i)
		s, err := newSim(sn, *broker, *user, *pass, *tenant, *pk, *rate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "设备 %d 启动失败: %v\n", i, err)
			os.Exit(1)
		}
		sims = append(sims, s)
		go s.run()
		if i < 5 {
			fmt.Printf("  sim sn=%s\n", sn)
		}
	}
	fmt.Printf("devicesim: %d 台全部运行", *n)
	if *dur > 0 {
		fmt.Printf(" %s\n", *dur)
		go func() {
			time.Sleep(*dur)
			close(stop)
		}()
	} else {
		fmt.Println("，Ctrl+C 停止")
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(stop)
	}
	for _, s := range sims {
		s.client.Disconnect(250)
	}
	fmt.Println("devicesim: 已停止")
}

func newSim(sn, broker, user, pass string, tenant int64, pk string, rate int) (*deviceSim, error) {
	opts := paho.NewClientOptions().
		AddBroker(broker).
		SetClientID("devicesim-" + sn).
		SetUsername(user).
		SetPassword(pass).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second)
	cli := paho.NewClient(opts)
	if token := cli.Connect(); !token.WaitTimeout(10*time.Second) || token.Error() != nil {
		return nil, fmt.Errorf("连接失败: %v", token.Error())
	}
	// 订阅下行指令（平台 down/{sn}/cmd）→ 回 ack
	downTopic := "down/" + sn + "/cmd"
	ackTopic := fmt.Sprintf("up/%d/%s/%s/ack", tenant, pk, sn)
	cli.Subscribe(downTopic, 1, func(_ paho.Client, m paho.Message) {
		var cmd struct {
			CmdID string `json:"cmd_id"`
			Type  string `json:"type"`
		}
		_ = json.Unmarshal(m.Payload(), &cmd)
		ack, _ := json.Marshal(map[string]any{
			"tenant_id": tenant, "sn": sn, "cmd_id": cmd.CmdID,
			"ok": true, "ts": time.Now().UnixMilli(),
		})
		cli.Publish(ackTopic, 1, false, ack)
	})
	return &deviceSim{
		sn:     sn,
		topic:  fmt.Sprintf("up/%d/%s/%s/telemetry", tenant, pk, sn),
		client: cli,
		rate:   rate,
		stop:   make(chan struct{}),
	}, nil
}

func (s *deviceSim) run() {
	interval := time.Second / time.Duration(max(s.rate, 1))
	t := time.NewTicker(interval)
	defer t.Stop()
	phase := mrand.Float64() * 2 * math.Pi
	for range t.C {
		phase += 0.1
		// 五指标模拟波形（SOC 缓降/充电回升循环 + 噪声）
		soc := 60 + 30*math.Sin(phase/50) + mrand.NormFloat64()
		payload, _ := json.Marshal(map[string]any{
			"soc":         round1(clamp(soc, 0, 100)),
			"voltage":     round1(48 + 2*mrand.NormFloat64()),
			"current":     round1(10 + 5*math.Sin(phase) + mrand.NormFloat64()),
			"temperature": round1(30 + 5*math.Sin(phase/10) + mrand.NormFloat64()),
			"power":       round1(480 + 240*math.Sin(phase) + mrand.NormFloat64()*20),
			"ts":          time.Now().UnixMilli(),
		})
		s.client.Publish(s.topic, 1, false, payload)
	}
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func round1(v float64) float64        { return math.Round(v*10) / 10 }
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var _ = sync.Once{}
