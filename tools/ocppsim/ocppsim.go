// ocppsim · OCPP 1.6J 模拟充电桩（S6-03，FR-IOT-008 验收口径=协议标准+模拟桩全链）。
//
//	go run ./tools/ocppsim -n 3 -dur 60s   # 3 桩 60s：BootNotification → Heartbeat/StatusNotification/MeterValues
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	n := flag.Int("n", 2, "模拟桩数")
	dur := flag.Duration("dur", 0, "运行时长（0=直到 Ctrl+C）")
	addr := flag.String("addr", envOr("OCPP_ADDR", "127.0.0.1:8182"), "CSMS 地址（iotingest）")
	flag.Parse()

	fmt.Printf("ocppsim: %d 桩 → ws://%s/ocpp/<cpID>\n", *n, *addr)
	dial := func(cpID string) *websocket.Conn {
		u := url.URL{Scheme: "ws", Host: *addr, Path: "/ocpp/" + cpID}
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "桩 %s 连接失败: %v\n", cpID, err)
			os.Exit(1)
		}
		// OCPP 1.6 子协议（Dial 子协议参数省略时服务端 CheckOrigin 已放行）
		return conn
	}

	stop := make(chan struct{})
	if *dur > 0 {
		go func() {
			time.Sleep(*dur)
			close(stop)
		}()
	} else {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		close(stop)
	}

	done := make(chan struct{}, *n)
	for i := 0; i < *n; i++ {
		cpID := fmt.Sprintf("CP-%03d", i+1)
		go func() {
			defer func() { done <- struct{}{} }()
			runPoint(cpID, dial, stop)
		}()
		time.Sleep(300 * time.Millisecond)
	}
	for i := 0; i < *n; i++ {
		<-done
	}
	fmt.Println("ocppsim: 已停止")
}

func runPoint(cpID string, dial func(string) *websocket.Conn, stop chan struct{}) {
	conn := dial(cpID)
	defer func() { _ = conn.Close() }()

	send := func(action string, payload any) {
		p, _ := json.Marshal(payload)
		frame, _ := json.Marshal([]any{2, uniqID(), action, json.RawMessage(p)})
		if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			fmt.Fprintf(os.Stderr, "桩 %s 发送失败: %v\n", cpID, err)
			return
		}
		// 同步等响应（CALL_RESULT）
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, resp, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "桩 %s 响应超时: %v\n", cpID, err)
			return
		}
		fmt.Printf("[%s] %s → %.80s\n", cpID, action, strings.ReplaceAll(string(resp), "\n", ""))
	}

	// 1. BootNotification
	send("BootNotification", map[string]any{
		"chargePointModel": "SIM-POC-16", "chargePointVendor": "micro-sim",
		"chargePointSerialNumber": cpID, "firmwareVersion": "1.0.0",
	})

	hb := time.NewTicker(10 * time.Second)
	mv := time.NewTicker(3 * time.Second)
	defer hb.Stop()
	defer mv.Stop()

	for {
		select {
		case <-stop:
			return
		case <-hb.C:
			send("Heartbeat", map[string]any{})
		case <-mv.C:
			// MeterValues：采样值 → 五指标（iotingest 映射 power/voltage/current）
			send("MeterValues", map[string]any{
				"connectorId": 1, "transactionId": 0,
				"meterValue": []map[string]any{{
					"timestamp": time.Now().UTC().Format(time.RFC3339),
					"sampledValue": []map[string]any{
						{"value": fmt.Sprintf("%.1f", 4800+rand.Float64()*200), "measurand": "Power.Active.Import", "unit": "W"},
						{"value": fmt.Sprintf("%.1f", 48+rand.Float64()), "measurand": "Voltage", "unit": "V"},
						{"value": fmt.Sprintf("%.1f", 10+rand.Float64()*5), "measurand": "Current.Import", "unit": "A"},
						{"value": fmt.Sprintf("%.1f", 30+rand.Float64()*10), "measurand": "Temperature", "unit": "Celsius"},
					},
				}},
			})
		}
		// 偶发 StatusNotification
		if rand.Float64() < 0.05 {
			send("StatusNotification", map[string]any{
				"connectorId": 1, "status": "Charging", "errorCode": "NoError",
			})
		}
	}
}

var seq int64

func uniqID() string {
	seq++
	return fmt.Sprintf("sim-%d-%d", time.Now().UnixNano(), seq)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
