// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.2

package notification

import (
	"net/http"

	"github.com/zeromicro/go-zero/rest/httpx"
	"micro-server/services/admin-bff/internal/logic/notification"
	"micro-server/services/admin-bff/internal/svc"
)

// 我的通知设置(perm: notification:setting:list)
func ListNotifySettingsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := notification.NewListNotifySettingsLogic(r.Context(), svcCtx)
		resp, err := l.ListNotifySettings()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
