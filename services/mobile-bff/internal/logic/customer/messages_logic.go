// messages_logic.go · 我的消息（user_id=uid 本人；未读计数随分页返回）。

package customer

import (
	"context"

	"micro-server/services/mobile-bff/internal/svc"
	"micro-server/services/mobile-bff/internal/types"
	npb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type MessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessagesLogic {
	return &MessagesLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MessagesLogic) Messages(req *types.MessageListReq) (resp *types.MessageListResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	out, err := l.svcCtx.Notify.ListMessages(l.ctx, &npb.ListMessagesReq{
		UserId:     uid,
		OnlyUnread: req.OnlyUnread,
		Page:       req.Page,
		Size:       req.Size,
	})
	if err != nil {
		return nil, err
	}
	unread, _ := l.svcCtx.Notify.UnreadCount(l.ctx, &npb.UnreadCountReq{UserId: uid})
	resp = &types.MessageListResp{Total: out.Total, UnreadCount: unread.GetCount(), List: []types.MessageView{}}
	for _, m := range out.List {
		resp.List = append(resp.List, pbMessage(m))
	}
	return resp, nil
}
