// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package org

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/org"
	"micro-server/services/admin-bff/internal/svc"
)

// 组织树（perm: system:org:list）
func ListOrgsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := org.NewListOrgsLogic(r.Context(), svcCtx)
		resp, err := l.ListOrgs()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
