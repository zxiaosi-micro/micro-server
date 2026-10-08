package logic

import (
	"context"

	"micro-server/services/party/internal/svc"
	"micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zxiaosi-micro/micro-common/errcode"
)

type ListContactLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListContactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListContactLogic {
	return &ListContactLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListContactLogic) ListContact(in *pb.ListContactReq) (*pb.ListContactResp, error) {
	tid, err := mustTenant(l.ctx)
	if err != nil {
		return nil, err
	}
	list, err := l.svcCtx.Models.Contact.ListByParty(l.ctx, tid, in.PartyId)
	if err != nil {
		return nil, errcode.Internal.WithCause(err)
	}
	resp := &pb.ListContactResp{}
	for _, c := range list {
		item := &pb.ContactItem{
			ContactId:  c.ContactId,
			PartyId:    c.PartyId,
			Name:       c.Name,
			Position:   c.Position.String,
			IsDefault:  c.IsDefault == 1,
			NotifyPref: c.NotifyPref.String,
			CreatedAt:  c.CreatedAt.UnixMilli(),
		}
		// 解密失败不出明文也不中断列表（历史数据/密钥轮换窗口）
		if plain, derr := l.svcCtx.Encryptor.Decrypt(c.Mobile); derr == nil {
			item.Mobile = string(plain)
		}
		resp.List = append(resp.List, item)
	}
	return resp, nil
}
