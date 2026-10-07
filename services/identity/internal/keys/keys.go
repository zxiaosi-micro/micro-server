// Package keys 装配 JWT 签发/验签密钥与数据加密密钥（S3-02）。
//
// 密钥来源（keygen 产出，S2-04；目录不入库，E11）：
//   - JWT：KeysDir 下 keys.json（kid 索引）+ jwt_rs256_private/public.pem。
//     轮换期（runbook RotateKey 流程）：新钥生成后旧公钥 PEM 留目录、
//     keys.json 拆出多公钥集合（本包先支持单钥 + 显式集合扩展点）。
//   - 数据：MICRO_DATA_KEYS="kid=<base64url 32B>,..." + MICRO_DATA_KEY_KID。
package keys

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zxiaosi-micro/micro-common/crypto"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
)

// keysFile keygen 产出的密钥索引（deploy/conf/keys/keys.json）。
type keysFile struct {
	JWT struct {
		KID     string `json:"kid"`
		Alg     string `json:"alg"`
		Private string `json:"private"`
		Public  string `json:"public"`
	} `json:"jwt"`
}

// LoadJWT 装配 Signer（identity 签发方）与 Verifier 公钥集合。
// keysDir 为空时回退显式 PEM 路径（单钥模式，测试用）。
func LoadJWT(keysDir, privateKeyFile, publicKeyFile string) (*jwtauth.Signer, *jwtauth.Verifier, error) {
	if keysDir == "" {
		if privateKeyFile == "" || publicKeyFile == "" {
			return nil, nil, errors.New("keys: Jwt.KeysDir 或显式 PEM 路径必须配置其一")
		}
		// 显式 PEM 单钥模式缺可靠 kid 源（kid 是验签路由唯一依据）——禁止，用 KeysDir
		return nil, nil, errors.New("keys: 显式 PEM 单钥模式缺 kid，请使用 Jwt.KeysDir（keys.json 提供 kid）")
	}

	raw, err := os.ReadFile(filepath.Join(keysDir, "keys.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("keys: 读 keys.json 失败: %w", err)
	}
	var kf keysFile
	if err := json.Unmarshal(raw, &kf); err != nil {
		return nil, nil, fmt.Errorf("keys: 解析 keys.json 失败: %w", err)
	}
	if kf.JWT.KID == "" || kf.JWT.Private == "" {
		return nil, nil, errors.New("keys: keys.json 缺少 jwt.kid / jwt.private")
	}

	priv, err := parsePriv(filepath.Join(keysDir, kf.JWT.Private))
	if err != nil {
		return nil, nil, err
	}
	signer, err := jwtauth.NewSigner(kf.JWT.KID, priv)
	if err != nil {
		return nil, nil, err
	}

	pubName := kf.JWT.Public
	if pubName == "" {
		pubName = "jwt_rs256_public.pem"
	}
	pub, err := parsePub(filepath.Join(keysDir, pubName))
	if err != nil {
		return nil, nil, err
	}
	verifier, err := jwtauth.NewVerifier(map[string]*rsa.PublicKey{kf.JWT.KID: pub})
	if err != nil {
		return nil, nil, err
	}
	return signer, verifier, nil
}

// LoadDataEncryptor 装配数据加密器（MICRO_DATA_KEYS / MICRO_DATA_KEY_KID 环境变量注入）。
func LoadDataEncryptor() (*crypto.Encryptor, error) {
	provider, err := crypto.EnvKeyProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("keys: 数据密钥未就绪（keygen 产出后注入 MICRO_DATA_KEYS）: %w", err)
	}
	return crypto.NewEncryptor(provider)
}

// HMACKey 取 HMAC-SHA256 索引键（mobile_hash/email_hash；默认与 seed 一致的 dev 键）。
func HMACKey(envName string) []byte {
	if envName == "" {
		envName = "MICRO_HASH_KEY"
	}
	if v := os.Getenv(envName); v != "" {
		return []byte(v)
	}
	return []byte("micro-dev-index-key")
}

func parsePriv(path string) (*rsa.PrivateKey, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("keys: 读私钥失败: %w", err)
	}
	return jwtauth.ParsePrivateKeyPEM(pem)
}

func parsePub(path string) (*rsa.PublicKey, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("keys: 读公钥失败: %w", err)
	}
	return jwtauth.ParsePublicKeyPEM(pem)
}
