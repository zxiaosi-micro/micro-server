package logic

import (
	"time"

	"github.com/zxiaosi-micro/micro-common/jwtauth"

	"micro-server/services/identity/internal/svc"
)

// jwtauthClaims 构造签发 claims（level=1；step-up 重签显式传 2）。
func jwtauthClaims(sc *svc.ServiceContext, uid int64, sid, client string) jwtauth.Claims {
	return jwtauth.Claims{UID: uid, SID: sid, Client: client, Level: 1}
}

// accessTTL access 有效期（config 注入，默认 30min）。
func accessTTL(sc *svc.ServiceContext) time.Duration {
	return time.Duration(sc.AccessTTL) * time.Second
}

// safePrefix 敏感令牌日志前缀（防全量泄漏）。
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
