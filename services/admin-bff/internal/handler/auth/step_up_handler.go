// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package auth

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/auth"
	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
)

// 二次认证提级（5min 窗口 level=2）
func StepUpHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StepUpReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := auth.NewStepUpLogic(r.Context(), svcCtx)
		resp, err := l.StepUp(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
