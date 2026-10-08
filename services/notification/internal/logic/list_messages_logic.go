package logic

import (
	"context"

	"micro-server/services/notification/internal/svc"
	"micro-server/services/notification/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListMessagesLogic {
	return &ListMessagesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListMessagesLogic) ListMessages(in *pb.ListMessagesReq) (*pb.ListMessagesResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	if in.UserId <= 0 {
		return nil, errUserRequired
	}
	page, size := clampPage(in.Page, in.Size)
	list, total, err := l.svcCtx.Models.Message.FindPageByUser(l.ctx, tid, in.UserId, in.OnlyUnread, page, size)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListMessagesResp{Total: total}
	for _, m := range list {
		resp.List = append(resp.List, &pb.MessageItem{
			MessageId: m.MessageId,
			UserId:    m.UserId,
			Title:     m.Title,
			Content:   m.Content,
			IsRead:    m.IsRead == 1,
			BizType:   m.BizType.String,
			BizId:     m.BizId.String,
			CreatedAt: m.CreatedAt.UnixMilli(),
		})
	}
	return resp, nil
}
