// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package role

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/role"
	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
)

// 新建角色（perm: system:role:create）
func CreateRoleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RoleCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := role.NewCreateRoleLogic(r.Context(), svcCtx)
		resp, err := l.CreateRole(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
