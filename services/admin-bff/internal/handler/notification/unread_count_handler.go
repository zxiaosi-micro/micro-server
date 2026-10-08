// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package notification

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/notification"
	"micro-server/services/admin-bff/internal/svc"
)

// 未读数(perm: notification:message:list)
func UnreadCountHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := notification.NewUnreadCountLogic(r.Context(), svcCtx)
		resp, err := l.UnreadCount()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
