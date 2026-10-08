// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package notification

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	notificationPb "micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/ctxkit"
)

type MarkReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 标记已读(perm: notification:message:read)
func NewMarkReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkReadLogic {
	return &MarkReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkReadLogic) MarkRead(req *types.IDPath) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Notification.MarkRead(l.ctx, &notificationPb.MarkReadReq{
		MessageId: parseID(req.ID), UserId: ctxkit.UID(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
