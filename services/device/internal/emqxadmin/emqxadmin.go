// Package emqxadmin · EMQX 凭证开通 + 指令下行 MQTT 发布（S6-01，FR-IOT-002）。
//
// 两条链路：
//   - REST（Dashboard API v5）：一机一密写入内置认证数据库 + ACL（bootstrap.sh 同款端点，E6 幂等）；
//   - MQTT：平台后端账号发布 down/{sn}/cmd QoS1（指令下行）。
//
// 教训对齐：EMQX 5.8 REST 不收 Basic——先 POST /api/v5/login 换 Bearer（bootstrap.sh 同款）；
// 认证器随容器重建丢失（E6）→ 本包写入均幂等（PUT/已存在容忍）。
package emqxadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/zeromicro/go-zero/core/logx"
)

// Client EMQX 管理面 + MQTT 面客户端（配置缺省时各方法降级为 no-op，本地开发免 EMQX 启动）。
type Client struct {
	apiBase       string
	dashUser      string
	dashPass      string
	broker        string
	platformUser  string
	platformPass  string
	http          *http.Client
	mu            sync.Mutex
	token         string
	tokenExpireAt time.Time
	mqtt          paho.Client
	mqttOnce      sync.Once
}

// New 构造（apiBase/broker 为空 = 降级模式：Provision/Publish 不生效仅记日志）。
func New(apiBase, dashUser, dashPass, broker, platformUser, platformPass string) *Client {
	return &Client{
		apiBase:      strings.TrimRight(apiBase, "/"),
		dashUser:     dashUser,
		dashPass:     dashPass,
		broker:       broker,
		platformUser: platformUser,
		platformPass: platformPass,
		http:         &http.Client{Timeout: 5 * time.Second},
	}
}

// Degraded 是否处于降级模式（未配置 EMQX）。
func (c *Client) Degraded() bool { return c.apiBase == "" }

// ---- REST 管理面 ----

// login 换 Bearer token（缓存至过期前 60s）。
func (c *Client) login(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpireAt.Add(-60*time.Second)) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"username": c.dashUser, "password": c.dashPass})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/api/v5/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Token    string `json:"token"`
		ExpireAt string `json:"expire_at"`
		IssuedAt string `json:"issued_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		return "", fmt.Errorf("emqx login 响应异常 http=%d err=%v", resp.StatusCode, err)
	}
	c.token = out.Token
	// EMQX token 有效期一般数小时；拿不到精确过期就保守 10min 重登
	c.tokenExpireAt = time.Now().Add(10 * time.Minute)
	return c.token, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) (int, []byte, error) {
	token, err := c.login(ctx)
	if err != nil {
		return 0, nil, err
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiBase+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, nil
}

// ensureAuthenticator 幂等创建密码认证器（built_in_database）。
func (c *Client) ensureAuthenticator(ctx context.Context) error {
	code, data, err := c.do(ctx, http.MethodGet, "/api/v5/authentication", nil)
	if err != nil {
		return err
	}
	if code == 200 && strings.Contains(string(data), "built_in_database") {
		return nil
	}
	code, _, err = c.do(ctx, http.MethodPost, "/api/v5/authentication", map[string]any{
		"mechanism": "password_based", "backend": "built_in_database",
		"password_hash_algorithm": map[string]string{"name": "bcrypt"}, "user_id_type": "username",
	})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 && code != 204 {
		return fmt.Errorf("创建认证器 http=%d", code)
	}
	return nil
}

// ProvisionDevice 写入一机一密（幂等：已存在则更新密码）+ 设备 ACL
// （仅可发布 up/{tenant}/{pk}/{sn}/#、订阅 down/{sn}/#，02 §7.2 ACL 最小主题）。
func (c *Client) ProvisionDevice(ctx context.Context, sn, secret string, tenantID int64, productKey string) error {
	if c.Degraded() {
		logx.WithContext(ctx).Infof("emqxadmin: 未配置 EMQX，凭证开通降级跳过 sn=%s", sn)
		return nil
	}
	if err := c.ensureAuthenticator(ctx); err != nil {
		return fmt.Errorf("ensureAuthenticator: %w", err)
	}
	// 用户（409/400 = 已存在，走 PUT 更新密码保证幂等）
	code, _, err := c.do(ctx, http.MethodPost, "/api/v5/authentication/password_based:built_in_database/users",
		map[string]any{"user_id": sn, "password": secret, "is_superuser": false})
	if err != nil {
		return err
	}
	if code != 200 && code != 201 && code != 204 && code != 400 && code != 409 {
		return fmt.Errorf("创建设备凭证 http=%d", code)
	}
	if code == 400 || code == 409 {
		code, _, err = c.do(ctx, http.MethodPut,
			"/api/v5/authentication/password_based:built_in_database/users/"+sn,
			map[string]any{"user_id": sn, "password": secret, "is_superuser": false})
		if err != nil {
			return err
		}
		if code != 200 && code != 201 && code != 204 {
			return fmt.Errorf("更新设备凭证 http=%d", code)
		}
	}
	// ACL：PUT 全量覆盖 = 幂等
	upTopic := fmt.Sprintf("up/%d/%s/%s/#", tenantID, productKey, sn)
	acl := map[string]any{
		"username": sn,
		"rules": []map[string]any{
			{"permission": "allow", "action": "publish", "topic": upTopic},
			{"permission": "allow", "action": "publish", "topic": fmt.Sprintf("up/%d/%s/%s", tenantID, productKey, sn)},
			{"permission": "allow", "action": "subscribe", "topic": fmt.Sprintf("down/%s/#", sn)},
			{"permission": "deny", "action": "all", "topic": "#"},
		},
	}
	code, _, err = c.do(ctx, http.MethodPut,
		"/api/v5/authorization/sources/built_in_database/rules/users/"+sn, acl)
	if err != nil {
		return err
	}
	if code != 200 && code != 201 && code != 204 {
		return fmt.Errorf("写入设备 ACL http=%d", code)
	}
	return nil
}

// ---- MQTT 指令下行 ----

// publishDown 发布 down/{sn}/cmd（QoS1）；连接懒建立，复用断线自动重连。
func (c *Client) publishDown(ctx context.Context, sn, payload string) error {
	if c.broker == "" || c.platformUser == "" {
		logx.WithContext(ctx).Infof("mqtt: 未配置 broker，指令下行降级跳过 sn=%s payload=%s", sn, payload)
		return nil
	}
	c.mqttOnce.Do(func() {
		opts := paho.NewClientOptions().
			AddBroker(c.broker).
			SetClientID("device-svc-" + fmt.Sprint(time.Now().UnixNano()%100000)).
			SetUsername(c.platformUser).
			SetPassword(c.platformPass).
			SetAutoReconnect(true).
			SetConnectRetry(true).
			SetConnectRetryInterval(10 * time.Second).
			SetConnectTimeout(5 * time.Second)
		c.mqtt = paho.NewClient(opts)
	})
	if !c.mqtt.IsConnectionOpen() {
		if token := c.mqtt.Connect(); !token.WaitTimeout(5 * time.Second) {
			return fmt.Errorf("mqtt 连接超时")
		} else if err := token.Error(); err != nil {
			return fmt.Errorf("mqtt 连接失败: %w", err)
		}
	}
	token := c.mqtt.Publish("down/"+sn+"/cmd", 1, false, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt 发布超时 sn=%s", sn)
	}
	return token.Error()
}

// PublishDown 对外暴露指令下行（internal 包内调用）。
func (c *Client) PublishDown(ctx context.Context, sn, payload string) error {
	return c.publishDown(ctx, sn, payload)
}
