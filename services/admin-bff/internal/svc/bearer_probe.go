package svc

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/jwtauth"
)

// BearerProbe 免鉴权组（/auth/logout /auth/step-up）的 Bearer 探针：
// 有合法 token 则注入 uid/sid/client 到 ctx（不查会话、不判权限——鉴权组由 authz 全链处理）。
// 无 token/无效 token 不拦截（免鉴权组语义），由 handler 自行决定是否报错。
func BearerProbe(verifier *jwtauth.Verifier) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if len(auth) >= 8 && strings.EqualFold(auth[:7], "Bearer ") {
				if claims, err := verifier.Verify(strings.TrimSpace(auth[7:])); err == nil {
					ctx := ctxkit.WithUID(r.Context(), claims.UID)
					ctx = ctxkit.WithSID(ctx, claims.SID)
					ctx = ctxkit.WithClient(ctx, claims.Client)
					r = r.WithContext(ctx)
				}
			}
			next(w, r)
		}
	}
}
