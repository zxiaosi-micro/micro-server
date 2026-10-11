// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package app

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/mobile-bff/internal/logic/app"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
)

// 告警待办增量（10s 轮询，FR-OPS-010）
func AppPendingAlertsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AppPendingAlertsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := app.NewAppPendingAlertsLogic(r.Context(), svcCtx)
		resp, err := l.AppPendingAlerts(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
