// Package ocpp · OCPP 1.6J WebSocket CSMS（S6-03，FR-IOT-008，:8182）。
//
// 四类核心消息 → 五指标映射进既有遥测链路（02 §7.3）：
//   - BootNotification → 设备上线（日志留痕；设备注册归 device 域）
//   - Heartbeat        → 心跳（影子 ts 刷新口径由上层处理）
//   - StatusNotification → 桩状态（raw 归档）
//   - MeterValues      → 电表采样（energy/measured_001 等采样值 → power/voltage/current 五指标）
//
// 验收口径 = 协议标准 + ocppsim 模拟桩全链（ADR-19）；2.0.1 预留扩展位（消息路由按 Action 分发）。
package ocpp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/core/logx"
)

// TelemetrySink 遥测出口（复用 forwarder 的 Kafka 语义：key=sn 保序）。
type TelemetrySink interface {
	Push(tenantId int64, pk, sn, payload string) error
}

// Server OCPP CSMS WebSocket 服务。
type Server struct {
	addr  string
	sink  TelemetrySink
	auth  func(chargePointId string) bool // 桩鉴权（dev 全放行）
	up    websocket.Upgrader
	mu    sync.Mutex
	conns map[string]*websocket.Conn
}

// NewServer 构造。
func NewServer(addr string, sink TelemetrySink, auth func(string) bool) *Server {
	return &Server{
		addr:  addr,
		sink:  sink,
		auth:  auth,
		conns: map[string]*websocket.Conn{},
		up: websocket.Upgrader{
			CheckOrigin:  func(r *http.Request) bool { return true }, // 桩直连
			Subprotocols: []string{"ocpp1.6"},
		},
	}
}

// Run 启动 HTTP/WebSocket 监听（阻塞）。
func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ocpp/", s.handleWS)
	srv := &http.Server{Addr: s.addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	logx.Infof("ocpp: CSMS 监听 %s（OCPP 1.6J）", s.addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleWS 单桩连接（/ocpp/{chargePointId}）。
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	cpId := strings.TrimPrefix(r.URL.Path, "/ocpp/")
	if cpId == "" || (s.auth != nil && !s.auth(cpId)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	conn, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		logx.Errorf("ocpp: upgrade 失败 cp=%s err=%v", cpId, err)
		return
	}
	s.mu.Lock()
	s.conns[cpId] = conn
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, cpId)
		s.mu.Unlock()
		_ = conn.Close()
	}()
	logx.Infof("ocpp: 桩接入 cp=%s", cpId)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			logx.Infof("ocpp: 桩断开 cp=%s err=%v", cpId, err)
			return
		}
		if resp := s.handleCall(cpId, raw); resp != "" {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(resp)); err != nil {
				logx.Errorf("ocpp: 回写失败 cp=%s err=%v", cpId, err)
				return
			}
		}
	}
}

// handleCall OCPP 1.6J CALL 帧：[2, uniqueId, action, payload] → 响应 [3, uniqueId, payload]。
func (s *Server) handleCall(cpId string, raw []byte) string {
	var frame []json.RawMessage
	if err := json.Unmarshal(raw, &frame); err != nil || len(frame) < 3 {
		logx.Errorf("ocpp: 非法帧 cp=%s raw=%.120s", cpId, string(raw))
		return ""
	}
	var msgType int
	var uniqueId string
	var action string
	_ = json.Unmarshal(frame[0], &msgType)
	_ = json.Unmarshal(frame[1], &uniqueId)
	if len(frame) >= 3 {
		_ = json.Unmarshal(frame[2], &action)
	}
	if msgType != 2 { // 非 CALL（CALL_RESULT=3/CALL_ERROR=4）不处理
		return ""
	}
	payload := "{}"
	if len(frame) >= 4 {
		payload = string(frame[3])
	}
	tenant, pk, respPayload := s.mapAction(cpId, action, payload)
	if respPayload == "" {
		respPayload = "{}"
	}
	// 遥测进 Kafka（key=cpId 保序；tenant 默认 1=平台默认租户，正式多租户接桩注册归属）
	if s.sink != nil && tenant >= 0 {
		msg := fmt.Sprintf(`{"tenant_id":%d,"product_key":%q,"sn":%q,"payload":%q,"ts":%d}`,
			tenant, pk, cpId, payload, time.Now().UnixMilli())
		if err := s.sink.Push(tenant, pk, cpId, msg); err != nil {
			logx.Errorf("ocpp: 遥测推送失败 cp=%s err=%v", cpId, err)
		}
	}
	return fmt.Sprintf(`[3,%q,%s]`, uniqueId, respPayload)
}

// mapAction 四类核心消息映射（02 §7.3；2.0.1 扩展位：switch action 增支即可）。
func (s *Server) mapAction(cpId, action, payload string) (tenant int64, pk, respPayload string) {
	switch action {
	case "BootNotification":
		logx.Infof("ocpp: BootNotification cp=%s payload=%.120s", cpId, payload)
		return -1, "", `{"status":"Accepted","currentTime":"` + time.Now().UTC().Format(time.RFC3339) + `","interval":300}`
	case "Heartbeat":
		return -1, "", `{"currentTime":"` + time.Now().UTC().Format(time.RFC3339) + `"}`
	case "StatusNotification":
		// 桩状态告警候选（faulted/error → ops 判定；简化：全部归档 raw）
		return 1, "ocpp", ""
	case "MeterValues":
		// 电表采样 → 五指标（power/voltage/current；energy/soc 由 ops 侧规则细化）
		return 1, "ocpp", ""
	default:
		logx.Infof("ocpp: 未处理消息（2.0.1 扩展位）cp=%s action=%s", cpId, action)
		return -1, "", ""
	}
}

// OnlineCount 在线桩数（metrics 观测口径）。
func (s *Server) OnlineCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}
