// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package middleware

import "net/http"

// AuthzMiddleware goctl 脚手架占位：实际鉴权中间件在 svc 组装（authz.New(...).Handle，
// 即 ServiceContext.Authz），本结构未挂载任何路由，保留仅为满足 goctl 模板。
type AuthzMiddleware struct {
}

// NewAuthzMiddleware 构造（未使用的脚手架占位，见类型注释）。
func NewAuthzMiddleware() *AuthzMiddleware {
	return &AuthzMiddleware{}
}

func (m *AuthzMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO generate middleware implement function, delete after code implementation

		// Passthrough to next handler if need
		next(w, r)
	}
}
