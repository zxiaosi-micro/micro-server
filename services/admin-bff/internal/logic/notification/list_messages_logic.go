// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"
	"strconv"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type ListMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 消息列表(perm: notification:message:list)
func NewListMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMessagesLogic {
	return &ListMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListMessagesLogic) ListMessages(req *types.MessageListReq) (resp *types.MessageListResp, err error) {
	uid := parseID(req.UserID)
	if uid == 0 {
		uid = ctxkit.UID(l.ctx)
	}
	r, err := l.svcCtx.Notification.ListMessages(l.ctx, &notificationPb.ListMessagesReq{
		UserId: uid, OnlyUnread: req.OnlyUnread, Page: int64(req.Page), Size: int64(req.Size),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.MessageListResp{Total: r.Total}
	for _, m := range r.List {
		resp.List = append(resp.List, types.MessageItem{
			MessageID: strconv.FormatInt(m.MessageId, 10), UserID: strconv.FormatInt(m.UserId, 10),
			Title: m.Title, Content: m.Content, IsRead: m.IsRead,
			BizType: m.BizType, BizID: m.BizId, CreatedAt: m.CreatedAt,
		})
	}
	return resp, nil
}
