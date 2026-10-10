// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package analytics

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/analytics"
	"micro-server/services/admin-bff/internal/svc"
)

// 读模型全量重建(版本号+1, perm: analytics:recon:rebuild)
func RebuildReadModelHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := analytics.NewRebuildReadModelLogic(r.Context(), svcCtx)
		resp, err := l.RebuildReadModel()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
