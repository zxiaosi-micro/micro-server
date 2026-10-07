// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package session

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/session"
	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
)

// 踢下线（perm: system:session:kick；sid 定点 / uid+client 该端 / uid 全端）
func KickSessionsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionKickReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := session.NewKickSessionsLogic(r.Context(), svcCtx)
		resp, err := l.KickSessions(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
