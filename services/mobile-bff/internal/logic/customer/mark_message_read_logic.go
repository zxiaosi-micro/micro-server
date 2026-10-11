// mark_message_read_logic.go · 消息已读（notification 仅本人可标记）。

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

type MarkMessageReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkMessageReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkMessageReadLogic {
	return &MarkMessageReadLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MarkMessageReadLogic) MarkMessageRead(req *types.IDPath) (resp *types.SimpleResp, err error) {
	uid := ctxkit.UID(l.ctx)
	if uid == 0 {
		return nil, errcode.ErrTokenInvalid
	}
	if _, err := l.svcCtx.Notify.MarkRead(l.ctx, &npb.MarkReadReq{MessageId: parseI64(req.ID), UserId: uid}); err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
