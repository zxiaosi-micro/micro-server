// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package menu

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/menu"
	"micro-server/services/admin-bff/internal/svc"
)

// 菜单管理列表（perm: system:menu:list）
func ListMenusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := menu.NewListMenusLogic(r.Context(), svcCtx)
		resp, err := l.ListMenus()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
