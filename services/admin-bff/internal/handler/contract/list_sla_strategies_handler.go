// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package contract

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/contract"
	"micro-server/services/admin-bff/internal/svc"
)

// SLA 策略列表(perm: contract:sla:list)
func ListSlaStrategiesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := contract.NewListSlaStrategiesLogic(r.Context(), svcCtx)
		resp, err := l.ListSlaStrategies()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
