// 渠道网关（S5-03，FR-FIN-001；韧性按 02 §10 渠道行：5s 超时 / 查询类 2 次退避 / DoWithAcceptable 熔断语义留位）。
//
// 三种模式（confcenter 渠道开关叠加）：
//   - mock：dev 模拟网关（冒烟不依赖真实商户号）——pay_url 为模拟链接，ConfirmPayment 由模拟回调/手工确认驱动；
//   - wechat：微信支付 v3（JSAPI/Native 下单 RSA-SHA256 签名；回调验签 = 平台公钥验 Wechatpay-Signature；
//     资源解密 = APIv3Key AES-256-GCM）；
//   - alipay：支付宝 RSA2（下单参数签名；异步通知验签 = 公钥验 sorted-params 签名）。
//
// 渠道未配置时自动回落 mock（dev 零配置可用）；生产配置由 env 注入（E11，仓库零真实密钥）。

package svc

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"micro-server/services/finance/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

// payTimeout 渠道出站超时（02 §10：微信/支付宝 5s）。
const payTimeout = 5 * time.Second

// Gateway 渠道网关。
type Gateway struct {
	cfg config.Config
}

// NewGateway 构造（按配置探测可用渠道；未配置渠道回落 mock）。
func NewGateway(cfg config.Config) *Gateway {
	return &Gateway{cfg: cfg}
}

// Mode 网关模式描述（启动日志/排查用）。
func (g *Gateway) Mode() string {
	var modes []string
	modes = append(modes, "mock")
	if g.wechatReady() {
		modes = append(modes, "wechat-v3")
	}
	if g.alipayReady() {
		modes = append(modes, "alipay-rsa2")
	}
	return strings.Join(modes, "+")
}

func (g *Gateway) wechatReady() bool {
	c := g.cfg.Wechat
	return c.MchID != "" && c.MchSerialNo != "" && c.AppID != "" && c.MchPrivateKey != ""
}

func (g *Gateway) alipayReady() bool {
	c := g.cfg.Alipay
	return c.AppID != "" && c.AppPrivateKey != ""
}

// MockPayURL dev 模拟网关支付链接（BFF 模拟确认按钮 → ConfirmPayment source=MOCK）。
func (g *Gateway) MockPayURL(paymentNo string) string {
	return fmt.Sprintf("/api/v1/pay/mock/%s", paymentNo)
}

// WechatNativeReady 微信 v3 是否已配置（验签/下单走真实渠道）。
func (g *Gateway) WechatNativeReady() bool { return g.wechatReady() }

// AlipayReady 支付宝 RSA2 是否已配置。
func (g *Gateway) AlipayReady() bool { return g.alipayReady() }

// WechatNativeURL 微信 Native 下单（扫码支付；管理后台 Web 口径），返回 code_url。
// 未配置商户号 → mock URL（dev 模拟网关）。
func (g *Gateway) WechatNativeURL(ctx context.Context, paymentNo, amount string, description string) (string, error) {
	if !g.wechatReady() {
		return g.MockPayURL(paymentNo), nil
	}
	c := g.cfg.Wechat
	privKey, err := parseRSAPrivateKey(c.MchPrivateKey)
	if err != nil {
		return "", fmt.Errorf("微信商户私钥解析失败: %w", err)
	}
	body := map[string]any{
		"mchid":       c.MchID,
		"appid":       c.AppID,
		"description": description,
		"out_trade_no": paymentNo,
		"amount":      map[string]any{"total": yuanToFen(amount), "currency": "CNY"},
		"notify_url":  "/api/v1/callback/wechat", // APISIX 边缘回填绝对地址
	}
	raw, _ := json.Marshal(body)
	ts := fmt.Sprint(time.Now().Unix())
	nonce := randNonce()
	msg := "POST\n/v3/pay/transactions/native\n" + ts + "\n" + nonce + "\n" + string(raw) + "\n"
	sig, err := signRSA(privKey, msg)
	if err != nil {
		return "", err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		c.MchID, nonce, sig, ts, c.MchSerialNo)
	ctx2, cancel := context.WithTimeout(ctx, payTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx2, http.MethodPost, "https://api.mch.weixin.qq.com/v3/pay/transactions/native", strings.NewReader(string(raw)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("微信下单失败(渠道降级开关见 configcenter): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		CodeURL string `json:"code_url"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("微信下单失败(%d): %s", resp.StatusCode, out.Message)
	}
	return out.CodeURL, nil
}

// WechatVerifyCallback 微信 v3 回调验签（平台公钥 RSA-SHA256 over timestamp\nnonce\nbody\n）。
func (g *Gateway) WechatVerifyCallback(timestamp, nonce, signature, body string) error {
	c := g.cfg.Wechat
	if c.PlatPublicKey == "" {
		return errors.New("微信平台公钥未配置(无法验签)")
	}
	pub, err := parseRSAPublicKey(c.PlatPublicKey)
	if err != nil {
		return err
	}
	msg := timestamp + "\n" + nonce + "\n" + body + "\n"
	digest := sha256.Sum256([]byte(msg))
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("签名 base64 解码失败: %w", err)
	}
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

// WechatDecryptResource 回调资源解密（AES-256-GCM，associated_data/nonce 报文自带）。
func (g *Gateway) WechatDecryptResource(associatedData, nonce, ciphertextB64 string) ([]byte, error) {
	key := []byte(g.cfg.Wechat.APIv3Key)
	if len(key) != 32 {
		return nil, errors.New("APIv3Key 未配置或长度非法")
	}
	ct, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), ct, []byte(associatedData))
}

// AlipayPayURL 支付宝统一收单页面支付（RSA2 签名跳转链接）。
// 未配置 → mock URL（dev 模拟网关）。
func (g *Gateway) AlipayPayURL(ctx context.Context, paymentNo, amount, subject string) (string, error) {
	if !g.alipayReady() {
		return g.MockPayURL(paymentNo), nil
	}
	c := g.cfg.Alipay
	privKey, err := parseRSAPrivateKey(c.AppPrivateKey)
	if err != nil {
		return "", fmt.Errorf("支付宝应用私钥解析失败: %w", err)
	}
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.page.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  "/api/v1/callback/alipay",
		"biz_content": fmt.Sprintf(`{"out_trade_no":"%s","total_amount":"%s","subject":"%s","product_code":"FAST_INSTANT_TRADE_PAY"}`, paymentNo, amount, subject),
	}
	sign, err := signParamsRSA2(privKey, params)
	if err != nil {
		return "", err
	}
	params["sign"] = sign
	var sb strings.Builder
	sb.WriteString(c.Gateway)
	sb.WriteString("?")
	first := true
	for _, k := range sortedKeys(params) {
		if !first {
			sb.WriteString("&")
		}
		first = false
		sb.WriteString(k + "=" + urlEscape(params[k]))
	}
	return sb.String(), nil
}

// AlipayVerifyCallback 支付宝异步通知验签（RSA2 over 排序参数，剔除 sign/sign_type）。
func (g *Gateway) AlipayVerifyCallback(form map[string]string) error {
	c := g.cfg.Alipay
	if c.AlipayPublicKey == "" {
		return errors.New("支付宝公钥未配置(无法验签)")
	}
	pub, err := parseRSAPublicKey(c.AlipayPublicKey)
	if err != nil {
		return err
	}
	sign := form["sign"]
	delete(form, "sign")
	delete(form, "sign_type")
	var pairs []string
	for _, k := range sortedKeys(form) {
		if form[k] == "" {
			continue
		}
		pairs = append(pairs, k+"="+form[k])
	}
	msg := strings.Join(pairs, "&")
	digest := sha256.Sum256([]byte(msg))
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return fmt.Errorf("签名 base64 解码失败: %w", err)
	}
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig)
}

// ---- 工具 ----

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("PEM 解码失败")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k8, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k8.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("非 RSA 私钥")
	}
	return rk, nil
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("PEM 解码失败")
	}
	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PublicKey); ok {
			return rk, nil
		}
	}
	if k, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("非 RSA 公钥")
}

func signRSA(k *rsa.PrivateKey, msg string) (string, error) {
	digest := sha256.Sum256([]byte(msg))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// signParamsRSA2 支付宝 RSA2：参数按 key 字典序拼 k=v&… 后签名。
func signParamsRSA2(k *rsa.PrivateKey, params map[string]string) (string, error) {
	var pairs []string
	for _, key := range sortedKeys(params) {
		if params[key] == "" {
			continue
		}
		pairs = append(pairs, key+"="+params[key])
	}
	return signRSA(k, strings.Join(pairs, "&"))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func urlEscape(s string) string {
	r := strings.NewReplacer("+", "%20", "&", "%26", "=", "%3D", "?", "%3F", "#", "%23", "%", "%25")
	return r.Replace(s)
}

func randNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func yuanToFen(yuan string) int64 {
	var yuanPart, fenPart int64
	i := strings.IndexByte(yuan, '.')
	if i < 0 {
		_, _ = fmt.Sscanf(yuan, "%d", &yuanPart)
		return yuanPart * 100
	}
	_, _ = fmt.Sscanf(yuan[:i], "%d", &yuanPart)
	frac := yuan[i+1:]
	if len(frac) == 1 {
		frac += "0"
	}
	_, _ = fmt.Sscanf(frac, "%02d", &fenPart)
	return yuanPart*100 + fenPart
}

var _ = logx.Info // 引用位（渠道日志走调用侧）
