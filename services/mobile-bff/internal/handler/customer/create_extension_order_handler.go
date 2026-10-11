// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package customer

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/mobile-bff/internal/logic/customer"
	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
)

// 延保购买（下单 + 微信支付，dev mock URL）
func CreateExtensionOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateExtensionOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := customer.NewCreateExtensionOrderLogic(r.Context(), svcCtx)
		resp, err := l.CreateExtensionOrder(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
