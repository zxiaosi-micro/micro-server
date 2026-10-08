// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package finance

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/finance"
	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
)

// 对账任务处理(仅 REPLAY/IGNORE, perm: finance:reconcile:resolve)
func ResolveReconcileTaskHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReconcileResolveReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := finance.NewResolveReconcileTaskLogic(r.Context(), svcCtx)
		resp, err := l.ResolveReconcileTask(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
