// Code scaffolded by goctl. Safe to edit.（S4-05 实现：BFF 仅做转发 + string↔int64（E8））

package party

import (
	"context"

	"micro-server/services/admin-bff/internal/svc"
	"micro-server/services/admin-bff/internal/types"
	partyPb "micro-server/services/party/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddContactLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 新增联系人(perm: party:contact:create)
func NewAddContactLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddContactLogic {
	return &AddContactLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddContactLogic) AddContact(req *types.ContactCreateReq) (resp *types.SimpleResp, err error) {
	_, err = l.svcCtx.Party.AddContact(l.ctx, &partyPb.AddContactReq{
		PartyId: parseID(req.Id), Name: req.Name, Mobile: req.Mobile,
		Position: req.Position, IsDefault: req.IsDefault, NotifyPref: req.NotifyPref,
	})
	if err != nil {
		return nil, err
	}
	return &types.SimpleResp{}, nil
}
