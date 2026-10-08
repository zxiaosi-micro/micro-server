// 文件对象与令牌核心：oss_key 规范、双桶路由、HMAC 下载令牌（02 §6.5）。

package logic

import (
	"context"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"micro-server/services/file/internal/confcenter"
	"micro-server/services/file/internal/svc"

	"github.com/minio/minio-go/v7"
)

// maxFileSize 上传大小上限（RPC bytes 通道 dev 口径 16MB；大文件直传属 S9）。
const maxFileSize = 16 << 20

// bucketFor biz_type 路由双桶：contract → 合同桶（版本控制），其余 → 普通桶。
func bucketFor(sc *svc.ServiceContext, bizType string) string {
	if bizType == "contract" {
		return sc.Config.Minio.BucketContract
	}
	return sc.Config.Minio.BucketDefault
}

// buildOssKey oss_key = /{tenant}/{biz_type}/{yyyy}/{mm}/{uuid}.{ext}（02 §6.5 规范）。
func buildOssKey(tid int64, bizType, name string) string {
	ext := ""
	if i := strings.LastIndex(name, "."); i >= 0 && i < len(name)-1 {
		ext = strings.ToLower(name[i:]) // 含点
	}
	now := time.Now()
	return fmt.Sprintf("/%d/%s/%04d/%02d/%s%s", tid, bizType, now.Year(), int(now.Month()), newUUID(), ext)
}

// newUUID 简化 UUID（crypto/rand hex，无库依赖）。
func newUUID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// parseInt64 字符串 → int64。
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// putObject 上传对象（上传开关 confcenter 热控）。
func putObject(ctx context.Context, sc *svc.ServiceContext, bucket, key, contentType string, data []byte) error {
	if !confcenter.Current().UploadEnabled {
		return errUploadOff
	}
	if len(data) > maxFileSize {
		return errFileTooLarge
	}
	_, err := sc.Minio.PutObject(ctx, bucket, key, strings.NewReader(string(data)), int64(len(data)),
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return errOssPut.WithCause(err)
	}
	return nil
}

// getObject 拉取对象内容（BFF 代理流式回源前先整体读，dev 规模适用）。
func getObject(ctx context.Context, sc *svc.ServiceContext, bucket, key string) ([]byte, error) {
	obj, err := sc.Minio.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, errOssGet.WithCause(err)
	}
	defer func() { _ = obj.Close() }()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, errOssGet.WithCause(err)
	}
	return data, nil
}

// signToken HMAC 签发下载令牌 token = tenant:file_id:expire_unix:sig。
// 租户编入令牌：下载 URL 离开租户上下文后仍可完成租户过滤（fail-closed 不破口）。
func signToken(signKey []byte, tid, fileId int64, exp time.Time) string {
	msg := itoa(tid) + ":" + itoa(fileId) + ":" + itoa(exp.Unix())
	mac := hmac.New(sha256.New, signKey)
	mac.Write([]byte(msg))
	return msg + ":" + hex.EncodeToString(mac.Sum(nil))
}

// tokenClaims 令牌承载的声明。
type tokenClaims struct {
	TenantID int64
	FileID   int64
}

// verifyToken 校验下载令牌（过期/签名不符 → errTokenInvalid）。
func verifyToken(signKey []byte, token string) (tokenClaims, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 4 {
		return tokenClaims{}, errTokenInvalid
	}
	msg := parts[0] + ":" + parts[1] + ":" + parts[2]
	mac := hmac.New(sha256.New, signKey)
	mac.Write([]byte(msg))
	if !hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(parts[3])) {
		return tokenClaims{}, errTokenInvalid
	}
	exp, err := parseInt64(parts[2])
	if err != nil {
		return tokenClaims{}, errTokenInvalid
	}
	if time.Now().Unix() > exp {
		return tokenClaims{}, errTokenInvalid
	}
	tid, err := parseInt64(parts[0])
	if err != nil {
		return tokenClaims{}, errTokenInvalid
	}
	fid, err := parseInt64(parts[1])
	if err != nil {
		return tokenClaims{}, errTokenInvalid
	}
	return tokenClaims{TenantID: tid, FileID: fid}, nil
}

// tokenExpSec 令牌有效期（confcenter 热调兜底值，S4-05 首批消费项）。
func tokenExpSec(sc *svc.ServiceContext) time.Duration {
	if v := confcenter.Current().TokenExpSec; v > 0 {
		return time.Duration(v) * time.Second
	}
	return sc.DefaultTokenExp()
}
