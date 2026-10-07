// keygen · 密钥三件套生成器（S2-04）
//
// 产出（默认落到 micro-deploy/deploy/conf/keys/，该目录已 gitignore，E11）：
//
//	jwt_rs256_private.pem / jwt_rs256_public.pem   RS256（JWT 签发/验签，同时产出 kid）
//	ota_ed25519_private.pem / ota_ed25519_public.pem Ed25519（OTA 固件签名）
//	data_key.txt + keys.json                        AES-256-GCM 数据密钥（crypto.EnvKeyProvider 消费）
//
// 约定：
//   - kid = SHA-256(SPKI DER) 前 16 hex；JWT header.kid 与验签方多公钥集合按此对齐（jwtauth）。
//   - 数据密钥格式 = base64url(32B)，与 crypto MICRO_DATA_KEYS="kid=<base64url>" 解析一致（S1-05）。
//   - 已有 keys.json 时拒绝覆盖（防止误把轮换期在用密钥冲掉），-force 才允许重生成。
//
// 用法：go run ./tools/keygen [-out ../micro-deploy/deploy/conf/keys] [-force]
package main

import (
	"crypto/aes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type keyMeta struct {
	GeneratedAt string      `json:"generated_at"`
	JWT         jwtMeta     `json:"jwt"`
	OTA         otaMeta     `json:"ota"`
	DataKey     dataKeyMeta `json:"data_key"`
	Notes       []string    `json:"notes"`
}

type jwtMeta struct {
	KID     string `json:"kid"`
	Alg     string `json:"alg"`
	Private string `json:"private"`
	Public  string `json:"public"`
}

type otaMeta struct {
	Alg     string `json:"alg"`
	Private string `json:"private"`
	Public  string `json:"public"`
}

type dataKeyMeta struct {
	KID      string `json:"kid"`
	Encoding string `json:"encoding"`
	File     string `json:"file"`
	EnvUsage string `json:"env_usage"`
}

func main() {
	out := flag.String("out", "../micro-deploy/deploy/conf/keys", "密钥输出目录（gitignore）")
	force := flag.Bool("force", false, "已存在 keys.json 时强制重新生成")
	flag.Parse()

	dir := *out
	metaPath := filepath.Join(dir, "keys.json")
	if _, err := os.Stat(metaPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "keygen: %s 已存在（密钥轮换期在用，不覆盖）。确认要重生成请加 -force\n", metaPath)
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fatal("创建目录", err)
	}

	// ---- 1) RS256（JWT）----
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fatal("生成 RSA", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)})
	pubDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		fatal("序列化公钥", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	kid := kidFromDER(pubDER)

	// ---- 2) Ed25519（OTA 签名）----
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal("生成 Ed25519", err)
	}
	edPrivDER, err := x509.MarshalPKCS8PrivateKey(edPriv)
	if err != nil {
		fatal("序列化 Ed25519 私钥", err)
	}
	edPubDER, err := x509.MarshalPKIXPublicKey(edPub)
	if err != nil {
		fatal("序列化 Ed25519 公钥", err)
	}

	// ---- 3) AES-256 数据密钥（base64url 32B，与 crypto.ParseKeySpec 对齐）----
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		fatal("生成数据密钥", err)
	}
	if len(raw) != aes.BlockSize*2 {
		fatal("数据密钥长度", fmt.Errorf("期望 32B，得到 %dB", len(raw)))
	}
	dkID := "dk-" + hex.EncodeToString(randBytes(4))
	dkB64 := base64.RawURLEncoding.EncodeToString(raw)

	files := map[string][]byte{
		"jwt_rs256_private.pem":   privPEM,
		"jwt_rs256_public.pem":    pubPEM,
		"ota_ed25519_private.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: edPrivDER}),
		"ota_ed25519_public.pem":  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: edPubDER}),
		"data_key.txt":            []byte(dkB64 + "\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			fatal("写入 "+name, err)
		}
	}

	meta := keyMeta{
		GeneratedAt: time.Now().Format(time.RFC3339),
		JWT:         jwtMeta{KID: kid, Alg: "RS256", Private: "jwt_rs256_private.pem", Public: "jwt_rs256_public.pem"},
		OTA:         otaMeta{Alg: "Ed25519", Private: "ota_ed25519_private.pem", Public: "ota_ed25519_public.pem"},
		DataKey: dataKeyMeta{
			KID:      dkID,
			Encoding: "base64url(32B)",
			File:     "data_key.txt",
			EnvUsage: fmt.Sprintf("MICRO_DATA_KEYS=%s=%s", dkID, dkB64),
		},
		Notes: []string{
			"私钥永不入库（E11）：目录已 gitignore，仓库只存公钥样例（keys.example）。",
			"轮换：keygen -force 生成新 kid 后新旧并存，观察期移除旧钥（jwtauth 多公钥集合）。",
			"数据密钥轮换：MICRO_DATA_KEYS 可带多个 kid=值，MICRO_DATA_KEY_KID 指定当前加密 kid。",
		},
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(metaPath, metaJSON, 0o600); err != nil {
		fatal("写入 keys.json", err)
	}

	fmt.Printf("== keygen 完成 → %s\n", dir)
	fmt.Printf("JWT  kid=%s（jwt_rs256_*.pem，RS256）\n", kid)
	fmt.Printf("OTA  Ed25519（ota_ed25519_*.pem）\n")
	fmt.Printf("DATA kid=%s（data_key.txt，base64url 32B）\n", dkID)
	fmt.Printf("\n粘贴到 compose/dev/.env（或服务环境）：\n  %s\n  MICRO_DATA_KEY_KID=%s\n", meta.DataKey.EnvUsage, dkID)
}

func kidFromDER(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:8]) // 16 hex，URL 安全
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func fatal(step string, err error) {
	fmt.Fprintf(os.Stderr, "keygen: %s: %v\n", step, err)
	os.Exit(1)
}
